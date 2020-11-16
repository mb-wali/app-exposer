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
