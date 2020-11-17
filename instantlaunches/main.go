package instantlaunches

import (
	"github.com/jmoiron/sqlx"
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
	DB *sqlx.DB
}

// New returns a newly created *App.
func New(db *sqlx.DB) *App {
	return &App{
		DB: db,
	}
}

const getLatestDefaultQuery = `
    SELECT def.id,
           def.version,
           def.instant_launches AS mapping
      FROM default_instant_launches def
  ORDER BY def.version DESC
     LIMIT 1;
`

// GetLatestDefault returns the latest version of the default instant launches.
func (a *App) GetLatestDefault() (DefaultInstantLaunchMapping, error) {
	m := DefaultInstantLaunchMapping{}
	err := a.DB.Get(&m, getLatestDefaultQuery)
	return m, err
}

const getDefaultByVersionQuery = `
    SELECT def.id,
           def.version,
           def.instant_launches as mapping
      FROM default_instant_launches def
     WHERE def.version = ?
`

// GetDefaultByVersion returns a specific version of the default instant launches.
func (a *App) GetDefaultByVersion(version int) (DefaultInstantLaunchMapping, error) {
	m := DefaultInstantLaunchMapping{}
	err := a.DB.Get(&m, getDefaultByVersionQuery, version)
	return m, err
}

const listDefaultsQuery = `
    SELECT def.id,
           def.version,
           def.instant_launches as mapping
      FROM default_instant_launches def
`

// ListDefaults returns a list of all of the default instant launches, including their version.
func (a *App) ListDefaults() ([]DefaultInstantLaunchMapping, error) {
	m := []DefaultInstantLaunchMapping{}
	err := a.DB.Select(&m, listDefaultsQuery)
	return m, err
}
