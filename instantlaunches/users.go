package instantlaunches

import (
	"encoding/json"
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
     WHERE users.username = ?
  ORDER BY u.version DESC
     LIMIT 1
`

// UserMapping returns the user's instant launch mappings.
func (a *App) UserMapping(user string) (UserInstantLaunchMapping, error) {
	m := UserInstantLaunchMapping{}
	err := a.DB.Get(&m, userMappingQuery, user)
	return m, err
}

// GetUserMapping is the echo handler for the http API that returns the user's
// instant launch mappings.
func (a *App) GetUserMapping(c echo.Context) error {
	user := c.Param("user")
	m, err := a.UserMapping(user)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, m)
}

const updateUserMappingQuery = `
    UPDATE ONLY user_instant_launches AS def
            SET def.instant_launches = jsonb_object(?)
           FROM users
          WHERE def.version = (
              SELECT max(version)
                FROM user_instant_launches
          )
            AND def.user_id = users.id
            AND users.username = ?
          RETURNING def.instant_launches;
`

// UpdateUserMapping updates the the latest version of the user's custom
// instant launch mappings.
func (a *App) UpdateUserMapping(user string, update echo.Map) (echo.Map, error) {
	marshalled, err := json.Marshal(update)
	if err != nil {
		return nil, err
	}
	updated := echo.Map{}
	err = a.DB.QueryRowx(updateUserMappingQuery, marshalled, user).Scan(updated)
	return updated, err
}

// UpdateUserMappingHandler is the echo handler for the HTTP API that updates the user's
// instant launch mapping.
func (a *App) UpdateUserMappingHandler(c echo.Context) error {
	user := c.Param("user")
	newdefaults := echo.Map{}
	err := c.Bind(&newdefaults)
	if err != nil {
		return err
	}
	updated, err := a.UpdateUserMapping(user, newdefaults)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, updated)
}

const allUserMappingQuery = `
  SELECT u.id,
         u.version,
         u.instant_launches as mapping
    FROM user_instant_launches u
    JOIN users ON u.user_id = users.id
   WHERE user.username = ?
`

// AllUserMapping returns all of the user's instant launch mappings regardless of version.
func (a *App) AllUserMapping(user string) ([]UserInstantLaunchMapping, error) {
	m := []UserInstantLaunchMapping{}
	err := a.DB.Select(&m, userMappingQuery, user)
	return m, err
}

// GetAllUserMappings is the echo handler for the http API that returns the user's
// instant launch mappings.
func (a *App) GetAllUserMappings(c echo.Context) error {
	user := c.Param("user")
	m, err := a.AllUserMapping(user)
	if err != nil {
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

// UserMappingsByVersion returns a specific version of the user's instant launch mappings.
func (a *App) UserMappingsByVersion(user string, version int) (UserInstantLaunchMapping, error) {
	m := UserInstantLaunchMapping{}
	err := a.DB.Get(&m, userMappingsByVersionQuery, user, version)
	return m, err
}

// GetUserMappingsByVersion is the echo handler for the http API that returns a specific
// version of the user's instant launch mappings.
func (a *App) GetUserMappingsByVersion(c echo.Context) error {
	user := c.Param("user")
	version, err := strconv.ParseInt(c.Param("version"), 10, 0)
	if err != nil {
		return err
	}
	m, err := a.UserMappingsByVersion(user, int(version))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, m)
}

const updateUserMappingsByVersionQuery = `
    UPDATE ONLY user_instant_launches AS def
            SET def.instant_launches = jsonb_object(?)
           FROM users
          WHERE def.version = ?
            AND def.user_id = users.id
            AND users.username = ?
        RETURNING def.instant_launches;
`

// UpdateUserMappingsByVersion updates the user's instant launches for a specific version.
func (a *App) UpdateUserMappingsByVersion(user string, version int, update echo.Map) (echo.Map, error) {
	marshalled, err := json.Marshal(update)
	if err != nil {
		return nil, err
	}
	retval := echo.Map{}
	if err = a.DB.QueryRowx(updateUserMappingsByVersionQuery, marshalled, version, user).Scan(&retval); err != nil {
		return nil, err
	}
	return retval, nil
}

// UpdateUserMappingsByVersionHandler is the echo handler for the HTTP API that allows callers
// to update a user's instant launches for a specific version.
func (a *App) UpdateUserMappingsByVersionHandler(c echo.Context) error {
	user := c.Param("user")
	version, err := strconv.ParseInt(c.Param("version"), 10, 0)
	if err != nil {
		return err
	}
	update := echo.Map{}
	if err = c.Bind(&update); err != nil {
		return err
	}
	newversion, err := a.UpdateUserMappingsByVersion(user, int(version), update)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, newversion)
}
