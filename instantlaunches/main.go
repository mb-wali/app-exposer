// Package classification instantlaunches
//
// Documentation for the instant launches API.
//
//		Schemes: http
//		BasePath: /instantlaunches
//		Version: 1.0.0
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
	"database/sql/driver"
	"encoding/json"
	"fmt"

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
	Pattern    string          `json:"pattern"`
	Kind       string          `json:"kind"`
	Default    InstantLaunch   `json:"default"`
	Compatible []InstantLaunch `json:"compatible"`
}

// InstantLaunchMapping maps a pattern string to an InstantLaunchSelector.
type InstantLaunchMapping map[string]*InstantLaunchSelector

// Scan implements the Scan function for database values.
func (i InstantLaunchMapping) Scan(value interface{}) error {
	switch v := value.(type) {
	case []byte:
		json.Unmarshal(v, &i)
		return nil
	case string:
		json.Unmarshal([]byte(v), &i)
		return nil
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
}

// Value implements the Value() function for database values.
func (i InstantLaunchMapping) Value() (driver.Value, error) {
	return json.Marshal(&i)
}

// UserInstantLaunchMapping contains the user-specific set of pattern-to-instant-launch
// mappings that override the system-level default set of mappings.
type UserInstantLaunchMapping struct {
	ID      string               `json:"id" db:"id"`
	Version string               `json:"version" db:"version"`
	UserID  string               `json:"user_id" db:"user_id"`
	Mapping InstantLaunchMapping `json:"mapping" db:"mapping"`
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

	// swagger:route get /instantlaunches/defaults instantlaunches listDefaults
	//
	// Lists the global defaults for mapping files to instant launches.
	//
	//		Produces:
	//		- application/json
	//
	//		Schemes: http
	//
	//		Deprecated: false
	//
	//		Responses:
	//			default: errorResponse
	//			200: listAllDefaults
	instance.Group.GET("/defaults", instance.GetListDefaults)

	// swagger:route get /instantlaunches/defaults/latest instantlaunches latestDefaults
	//
	// Gets the latest default instant launch mapping.
	//
	// 		Produces:
	//		- application/json
	//
	//		Schemes: http
	//
	//		Deprecated: false
	//
	//		Responses:
	//			default: errorResponse
	//			200: defaultMapping
	instance.Group.GET("/defaults/latest", instance.GetLatestDefaults)

	// swagger:route put /instantlaunches/defaults/latest instantlaunches addLatestDefaults
	//
	// Adds a new latest defaults mapping.
	//
	//		Consumes:
	//		- application/json
	//
	//		Produces:
	//		- application/json
	//
	//		Schemes: http
	//
	//		Deprecated: false
	//
	//		Responses:
	//			default: errorResponse
	//			200: defaultMapping
	instance.Group.PUT("/defaults/latest", instance.AddLatestDefaultsHandler)

	// swagger:route post /instantlaunches/defaults/latest instantlaunches updateLatestDefaults
	//
	// Updates the latest defaults mapping.
	//
	//		Consumes:
	//		- application/json
	//
	//		Produces:
	//		- application/json
	//
	//		Schemes: http
	//
	//		Deprecated: false
	//
	//		Responses:
	//			default: errorResponse
	//			200: defaultMapping
	instance.Group.POST("/defaults/latest", instance.UpdateLatestDefaultsHandler)

	// swagger:route delete /instantlaunches/defaults/latest instantlaunches deleteLatestDefaults
	//
	// Deletes the latest defaults mapping.
	//
	//		Produces:
	//		- application/json
	//
	//		Schemes: http
	//
	//		Deprecated: false
	//
	//		Responses:
	//			default: errorResponse
	instance.Group.DELETE("/defaults/latest", instance.DeleteLatestDefaultsHandler)

	// swagger:route get /instantlaunches/defaults/{version} instantlaunches defaultsByVersion
	//
	// Gets the default instant launch mapping at the specified version.
	//
	// 		Produces:
	//		- application/json
	//
	//		Schemes: http
	//
	//		Deprecated: false
	//
	//		Responses:
	//			default: errorResponse
	//			200: defaultMapping
	instance.Group.GET("/defaults/:version", instance.DefaultsByVersionHandler)

	// swagger:route post /instantlaunches/defaults/{version} instantlaunches updateDefaultsByVersion
	//
	// Updates the default instant launch mapping at the specified version.
	//
	//		Consumes:
	//		- application/json
	//
	// 		Produces:
	//		- application/json
	//
	//		Schemes: http
	//
	//		Deprecated: false
	//
	//		Responses:
	//			default: errorResponse
	//			200: defaultMapping
	instance.Group.POST("/defaults/:version", instance.UpdateDefaultsByVersionHandler)

	// swagger:route delete /instantlaunches/defaults/{version} instantlaunches deleteDefaultsByVersion
	//
	// Deletes the default instant launch mapping at the specified version.
	//
	// 		Produces:
	//		- application/json
	//
	//		Schemes: http
	//
	//		Deprecated: false
	//
	//		Responses:
	//			default: errorResponse
	instance.Group.DELETE("/defaults/:version", instance.DeleteDefaultsByVersionHandler)

	// swagger:route get /instantlaunches/{username} instantlaunches getAllUserDefaults
	//
	// Lists the user-specific mappings of files to instant launches regardless of version.
	//
	//		Produces:
	//		- application/json
	//
	//		Schemes: http
	//
	//		Deprecated: false
	//
	//		Responses:
	//			default: errorResponse
	//			200: listAllDefaults
	instance.Group.GET("/:username", instance.AllUserMappingsHandler)

	// swagger:route get /instantlaunches/{username}/latest instantlaunches getUserDefaults
	//
	// Gets the latest user-specific instant launch mapping.
	//
	// 		Produces:
	//		- application/json
	//
	//		Schemes: http
	//
	//		Deprecated: false
	//
	//		Responses:
	//			default: errorResponse
	//			200: defaultMapping
	instance.Group.GET("/:username/latest", instance.UserMappingHandler)

	// swagger:route put /instantlaunches/{username}/latest instantlaunches addUserDefaults
	//
	// Adds a new user-specific defaults mapping.
	//
	//		Consumes:
	//		- application/json
	//
	//		Produces:
	//		- application/json
	//
	//		Schemes: http
	//
	//		Deprecated: false
	//
	//		Responses:
	//			default: errorResponse
	//			200: defaultMapping
	instance.Group.PUT("/:username", instance.AddUserMappingHandler)

	// swagger:route post /instantlaunches/{username}/latest instantlaunches updateUserDefaults
	//
	// Updates the latest user-specific defaults mapping.
	//
	//		Consumes:
	//		- application/json
	//
	//		Produces:
	//		- application/json
	//
	//		Schemes: http
	//
	//		Deprecated: false
	//
	//		Responses:
	//			default: errorResponse
	//			200: defaultMapping
	instance.Group.POST("/:username/latest", instance.UpdateUserMappingHandler)

	// swagger:route delete /instantlaunches/{username}/latest instantlaunches deleteUserDefaults
	//
	// Deletes the user-specific instant launch mapping.
	//
	// 		Produces:
	//		- application/json
	//
	//		Schemes: http
	//
	//		Deprecated: false
	//
	//		Responses:
	//			default: errorResponse
	instance.Group.DELETE("/:username/latest", instance.DeleteUserMappingHandler)

	// swagger:route get /instantlaunches/{username}/{version} instantlaunches getUserDefaultsByVersion
	//
	// Gets the latest user-specific instant launch mapping by version.
	//
	// 		Produces:
	//		- application/json
	//
	//		Schemes: http
	//
	//		Deprecated: false
	//
	//		Responses:
	//			default: errorResponse
	//			200: defaultMapping
	instance.Group.GET("/:username/:version", instance.UserMappingsByVersionHandler)

	// swagger:route post /instantlaunches/{username}/{version} instantlaunches updateUserDefaultsByVersion
	//
	// Updates a user-specific defaults mapping by version.
	//
	//		Consumes:
	//		- application/json
	//
	//		Produces:
	//		- application/json
	//
	//		Schemes: http
	//
	//		Deprecated: false
	//
	//		Responses:
	//			default: errorResponse
	//			200: defaultMapping
	instance.Group.POST("/:username/:version", instance.UpdateUserMappingsByVersionHandler)

	// swagger:route delete /instantlaunches/{username}/{version} instantlaunches deleteUSerDefaultsByVersion
	//
	// Deletes a user-specific instant launch mapping at the specified version.
	//
	// 		Produces:
	//		- application/json
	//
	//		Schemes: http
	//
	//		Deprecated: false
	//
	//		Responses:
	//			default: errorResponse
	instance.Group.DELETE("/:username/:version", instance.DeleteUserMappingsByVersionHandler)

	return instance
}

// swagger:parameters defaultsByVersion
type defaultsByVersionParams struct {
	// in: path
	// required: true
	// minimum: 0
	Version int
}
