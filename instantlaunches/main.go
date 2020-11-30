// Package classification instantlaunches
//
// Documentation for the instant launches API.
//
//		Schemes: http
//		BasePath: /instantlaunches
//		Version: 1.0.0
//		Host: app-exposer
//
//		Consumes:
//		- application/json
//
//		Produces:
//		- application/json
//
// swagger:meta

package instantlaunches

import (
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

	// swagger:route get /default instantlaunches defaults listDefaults
	instance.Group.GET("/default", instance.GetListDefaults)

	// swagger:route get /default/latest instantlaunches defaults latestDefaults
	instance.Group.GET("/default/latest", instance.GetLatestDefaults)

	// swagger:route get /default/{version} instantlaunches defaults defaultsByVersion
	instance.Group.GET("/default/:version", instance.GetDefaultsByVersion)
	return instance
}

// swagger:parameters defaultsByVersion
type defaultsByVersionParams struct {
	// in: path
	// required: true
	// minimum: 0
	Version int
}
