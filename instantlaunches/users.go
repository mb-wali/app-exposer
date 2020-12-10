package instantlaunches

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

const userMappingQuery = `
    SELECT u.id,
           u.version,
           u.instant_launches as mapping
      FROM user_instant_launches u
      JOIN users ON u.user_id = users.id
     WHERE users.username = $1
  ORDER BY u.version DESC
     LIMIT 1;
`

const updateUserMappingQuery = `
    UPDATE ONLY user_instant_launches
            SET user_instant_launches.instant_launches = $1
           FROM users
          WHERE user_instant_launches.version = (
              SELECT max(u.version)
                FROM user_instant_launches u
          )
            AND user_id = users.id
            AND users.username = $2
          RETURNING user_instant_launches.instant_launches;
`

const deleteUserMappingQuery = `
	DELETE FROM ONLY user_instant_launches
	USING users
	WHERE user_instant_launches.user_id = users.id
	  AND users.username = $1
	  AND user_instant_launches.version = (
		  SELECT max(u.version)
		    FROM user_instant_launches u
	  );
`

const createUserMappingQuery = `
	INSERT INTO user_instant_launches (instant_launches, user_id)
	VALUES ( $1, (SELECT id FROM users WHERE username = $2) )
	RETURNING instant_launches;
`

// UserMapping returns the user's instant launch mappings.
func (a *App) UserMapping(user string) (*UserInstantLaunchMapping, error) {
	m := &UserInstantLaunchMapping{}
	err := a.DB.Get(m, userMappingQuery, user)
	return m, err
}

// UserMappingHandler is the echo handler for the http API that returns the user's
// instant launch mappings.
func (a *App) UserMappingHandler(c echo.Context) error {
	user := c.Param("username")
	if user == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "user was not set")
	}

	m, err := a.UserMapping(user)
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}
		return err
	}
	return c.JSON(http.StatusOK, m)
}

// UpdateUserMapping updates the the latest version of the user's custom
// instant launch mappings.
func (a *App) UpdateUserMapping(user string, update *InstantLaunchMapping) (*InstantLaunchMapping, error) {
	updated := &InstantLaunchMapping{}
	err := a.DB.QueryRowx(updateUserMappingQuery, update, user).Scan(updated)
	return updated, err
}

// UpdateUserMappingHandler is the echo handler for the HTTP API that updates the user's
// instant launch mapping.
func (a *App) UpdateUserMappingHandler(c echo.Context) error {
	user := c.Param("username")
	if user == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "user was not set")
	}

	newdefaults, err := InstantLaunchMappingFromJSON(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot parse JSON")
	}

	updated, err := a.UpdateUserMapping(user, newdefaults)
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}
		return err
	}

	return c.JSON(http.StatusOK, updated)
}

// DeleteUserMapping is intended as an admin only operation that completely removes
// the latest mapping for the user.
func (a *App) DeleteUserMapping(user string) error {
	_, err := a.DB.Exec(deleteUserMappingQuery, user)
	return err
}

// DeleteUserMappingHandler is the handler for the admin-only operation that removes
// the latest mapping for the user.
func (a *App) DeleteUserMappingHandler(c echo.Context) error {
	user := c.Param("username")
	if user == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "user was not set")
	}
	return a.DeleteUserMapping(user)
}

// AddUserMapping adds a new record to the database for the user's instant launches.
func (a *App) AddUserMapping(user string, mapping *InstantLaunchMapping) (*InstantLaunchMapping, error) {
	newvalue := &InstantLaunchMapping{}
	err := a.DB.QueryRowx(createUserMappingQuery, mapping, user).Scan(newvalue)
	if err != nil {
		return nil, err
	}
	return newvalue, nil
}

// AddUserMappingHandler is the HTTP handler for adding a new user mapping to the database.
func (a *App) AddUserMappingHandler(c echo.Context) error {
	user := c.Param("username")

	newvalue, err := InstantLaunchMappingFromJSON(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot parse JSON")
	}

	retval, err := a.AddUserMapping(user, newvalue)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, retval)
}

const allUserMappingsQuery = `
  SELECT u.id,
		 u.version,
		 u.user_id,
         u.instant_launches as mapping
    FROM user_instant_launches u
    JOIN users ON u.user_id = users.id
   WHERE users.username = ?;
`

// AllUserMappings returns all of the user's instant launch mappings regardless of version.
func (a *App) AllUserMappings(user string) ([]UserInstantLaunchMapping, error) {
	m := []UserInstantLaunchMapping{}
	err := a.DB.Select(&m, allUserMappingsQuery, user)
	return m, err
}

// AllUserMappingsHandler is the echo handler for the http API that returns the user's
// instant launch mappings.
func (a *App) AllUserMappingsHandler(c echo.Context) error {
	user := c.Param("username")
	m, err := a.AllUserMappings(user)
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}
		return err
	}
	return c.JSON(http.StatusOK, m)
}

const userMappingsByVersionQuery = `
    SELECT u.id,
           u.version,
           u.instant_launches as mapping
      FROM user_instant_launches u
      JOIN users ON u.user_id = users.id
     WHERE users.username = ?
       AND u.version = ?
`

const updateUserMappingsByVersionQuery = `
    UPDATE ONLY user_instant_launches AS def
            SET def.instant_launches = jsonb_object(?)
           FROM users
          WHERE def.version = ?
            AND def.user_id = users.id
            AND users.username = ?
        RETURNING def.instant_launches;
`

const deleteUserMappingsByVersionQuery = `
	DELETE FROM ONLY user_instant_launches AS def
	USING users
	WHERE def.user_id = users.id
	  AND users.username = ?
	  AND def.version = ?;
`

// UserMappingsByVersion returns a specific version of the user's instant launch mappings.
func (a *App) UserMappingsByVersion(user string, version int) (UserInstantLaunchMapping, error) {
	m := UserInstantLaunchMapping{}
	err := a.DB.Get(&m, userMappingsByVersionQuery, user, version)
	return m, err
}

// UserMappingsByVersionHandler is the echo handler for the http API that returns a specific
// version of the user's instant launch mappings.
func (a *App) UserMappingsByVersionHandler(c echo.Context) error {
	user := c.Param("username")
	version, err := strconv.ParseInt(c.Param("version"), 10, 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot process version")
	}
	m, err := a.UserMappingsByVersion(user, int(version))
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}
		return err
	}
	return c.JSON(http.StatusOK, m)
}

// UpdateUserMappingsByVersion updates the user's instant launches for a specific version.
func (a *App) UpdateUserMappingsByVersion(user string, version int, update *InstantLaunchMapping) (*InstantLaunchMapping, error) {
	retval := &InstantLaunchMapping{}
	err := a.DB.QueryRowx(updateUserMappingsByVersionQuery, update, version, user).Scan(retval)
	if err != nil {
		return nil, err
	}
	return retval, nil
}

// UpdateUserMappingsByVersionHandler is the echo handler for the HTTP API that allows callers
// to update a user's instant launches for a specific version.
func (a *App) UpdateUserMappingsByVersionHandler(c echo.Context) error {
	user := c.Param("username")
	version, err := strconv.ParseInt(c.Param("version"), 10, 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot process version")
	}

	// I'm not sure why, but this stuff seems to break echo's c.Bind() function
	// so we handle the unmarshalling without it here.
	update, err := InstantLaunchMappingFromJSON(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot parse JSON")
	}

	newversion, err := a.UpdateUserMappingsByVersion(user, int(version), update)
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}
		return err
	}
	return c.JSON(http.StatusOK, newversion)
}

// DeleteUserMappingsByVersion deletes a user's instant launch mappings at a specific version.
func (a *App) DeleteUserMappingsByVersion(user string, version int) error {
	_, err := a.DB.Exec(deleteUserMappingsByVersionQuery, user, version)
	return err
}

// DeleteUserMappingsByVersionHandler is the echo handler for the HTTP API that allows callers
// delete a user's instant launch mappings at a specific version.
func (a *App) DeleteUserMappingsByVersionHandler(c echo.Context) error {
	user := c.Param("username")
	version, err := strconv.ParseInt(c.Param("version"), 10, 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot process version")
	}
	return a.DeleteUserMappingsByVersion(user, int(version))
}
