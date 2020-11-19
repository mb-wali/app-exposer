package instantlaunches

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

// InstantLaunch contains the information needed to instantly launch an app.
type InstantLaunch struct {
	ID            string `json:"id"`
	QuickLaunchID string `json:"quick_launch_id"`
	AddedBy       string `json:"added_by"` // short username
	AddedOn       string `json:"added_on"` // formatted timestamp including timezone
	db            *sqlx.DB
}

// InstantLaunchSelector defines the default and compatible instant launches
// for a file pattern.
type InstantLaunchSelector struct {
	Pattern    string
	Kind       string
	Default    InstantLaunch
	Compatible []InstantLaunch
}

// InstantLaunchMapping maps a pattern string to an InstantLaunchSelector.
type InstantLaunchMapping map[string]*InstantLaunchSelector

// DefaultInstantLaunchMapping contains the system level set of default pattern
// to instant launch mappings.
type DefaultInstantLaunchMapping struct {
	ID      string
	Version string // determines the format.
	Mapping InstantLaunchMapping
}

// UserInstantLaunchMapping contains the user-specific set of pattern-to-instant-launch
// mappings that override the system-level default set of mappings.
type UserInstantLaunchMapping struct {
	ID      string
	Version string
	UserID  string
	Mapping InstantLaunchMapping
}

// App provides an API for managing instant launches.
type App struct {
	DB    *sqlx.DB
	Group *echo.Group
}

// New returns a newly created *App.
func New(db *sqlx.DB, group *echo.Group) *App {
	instance := &App{
		DB:    db,
		Group: group,
	}

	instance.Group.GET("/default", instance.GetListDefaults)
	instance.Group.GET("/default/latest", instance.GetLatestDefaults)
	instance.Group.GET("/default/:version", instance.GetDefaultsByVersion)
	return instance
}

const latestDefaultsQuery = `
    SELECT def.id,
           def.version,
           def.instant_launches AS mapping
      FROM default_instant_launches def
  ORDER BY def.version DESC
     LIMIT 1;
`

// LatestDefaults returns the latest version of the default instant launches.
func (a *App) LatestDefaults() (DefaultInstantLaunchMapping, error) {
	m := DefaultInstantLaunchMapping{}
	err := a.DB.Get(&m, latestDefaultsQuery)
	return m, err
}

// GetLatestDefaults is the echo handler for the http API that returns the
// default mapping of instant launches to file patterns.
func (a *App) GetLatestDefaults(c echo.Context) error {
	defaults, err := a.LatestDefaults()
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, defaults)
}

const updateLatestDefaultsQuery = `
    UPDATE ONLY default_instant_launches AS def
            SET def.instant_launches = jsonb_object(?)
          WHERE def.version = (
              SELECT max(version)
                FROM default_instant_launches
          )
          RETURNING def.instant_launches;
`

// UpdateLatestDefaults sets a new value for the latest version of the defaults.
func (a *App) UpdateLatestDefaults(newjson echo.Map) (echo.Map, error) {
	marshalled, err := json.Marshal(newjson)
	if err != nil {
		return nil, err
	}
	retval := echo.Map{}
	err = a.DB.QueryRowx(updateLatestDefaultsQuery, marshalled).Scan(&retval)
	return retval, err
}

// UpdateLatestDefaultsHandler is the echo handler for the HTTP API that updates
// the latest defaults mapping.
func (a *App) UpdateLatestDefaultsHandler(c echo.Context) error {
	newdefaults := echo.Map{}
	err := c.Bind(&newdefaults)
	if err != nil {
		return err
	}
	updated, err := a.UpdateLatestDefaults(newdefaults)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, updated)
}

const defaultsByVersionQuery = `
    SELECT def.id,
           def.version,
           def.instant_launches as mapping
      FROM default_instant_launches def
     WHERE def.version = ?
`

// DefaultsByVersion returns a specific version of the default instant launches.
func (a *App) DefaultsByVersion(version int) (DefaultInstantLaunchMapping, error) {
	m := DefaultInstantLaunchMapping{}
	err := a.DB.Get(&m, defaultsByVersionQuery, version)
	return m, err
}

// GetDefaultsByVersion is the echo handler for the http API that returns the defaults
// stored for the provided format version.
func (a *App) GetDefaultsByVersion(c echo.Context) error {
	version, err := strconv.ParseInt(c.Param("version"), 10, 0)
	if err != nil {
		return err
	}

	m, err := a.DefaultsByVersion(int(version))
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, m)
}

const updateDefaultsByVersionQuery = `
    UPDATE ONLY default_instant_launches AS def
            SET def.instant_launches = jsonb_object(?)
          WHERE def.version = ?
      RETURNING def.instant_launches
`

// UpdateDefaultsByVersion updates the default mapping for a specific version.
func (a *App) UpdateDefaultsByVersion(newjson echo.Map, version int) (echo.Map, error) {
	marshalled, err := json.Marshal(newjson)
	if err != nil {
		return nil, err
	}
	updated := echo.Map{}
	err = a.DB.QueryRowx(updateDefaultsByVersionQuery, marshalled, version).Scan(updated)
	return updated, err
}

// UpdateDefaultsByVersionHandler is the echo handler for the HTTP API that
// updates the default mapping for a specific version.
func (a *App) UpdateDefaultsByVersionHandler(c echo.Context) error {
	version, err := strconv.ParseInt(c.Param("version"), 10, 0)
	if err != nil {
		return err
	}
	newvalue := echo.Map{}
	if err = c.Bind(&newvalue); err != nil {
		return err
	}
	updated, err := a.UpdateDefaultsByVersion(newvalue, int(version))
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, updated)
}

const listAllDefaultsQuery = `
    SELECT def.id,
           def.version,
           def.instant_launches as mapping
      FROM default_instant_launches def
`

// ListAllDefaults returns a list of all of the default instant launches, including their version.
func (a *App) ListAllDefaults() ([]DefaultInstantLaunchMapping, error) {
	m := []DefaultInstantLaunchMapping{}
	err := a.DB.Select(&m, listAllDefaultsQuery)
	return m, err
}

// GetListDefaults is the echo handler for the http API that returns a list of
// all defaults listed in the database, regardless of version.
func (a *App) GetListDefaults(c echo.Context) error {
	m, err := a.ListAllDefaults()
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, m)
}

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
