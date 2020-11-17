package instantlaunches

import (
	"net/http"
	"strconv"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo"
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
