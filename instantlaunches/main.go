package instantlaunches

import (
	"database/sql"
)

// InstantLaunch contains the information needed to instantly launch an app.
type InstantLaunch struct {
	ID            string `json:"id"`
	QuickLaunchID string `json:"quick_launch_id"`
	AddedBy       string `json:"added_by"` // short username
	AddedOn       string `json:"added_on"` // formatted timestamp including timezone
	db            *sql.DB
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
	DB *sql.DB
}

// New returns a newly created *App.
func New(db *sql.DB) *App {
	return &App{
		DB: db,
	}
}

// GetLatestDefault returns the latest version of the default instant launches.

// GetDefaultByVersion returns a specific version of the default instant launches.

// ListDefaults returns a list of all of the default instant launches, including their version.
