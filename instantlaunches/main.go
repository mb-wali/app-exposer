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
	"io"
	"io/ioutil"

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

// InstantLaunchMappingFromJSON creates a new *InstantLaunchMapping from the io.ReadCloser
// passed in. Calls ioutil.ReadAll on the ReadCloser and closes it.
func InstantLaunchMappingFromJSON(r io.ReadCloser) (*InstantLaunchMapping, error) {
	il := &InstantLaunchMapping{}

	defer r.Close()
	readbytes, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(readbytes, il); err != nil {
		return nil, err
	}

	return il, nil
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
	DB         *sqlx.DB
	Group      *echo.Group
	UserSuffix string
}

// New returns a newly created *App.
func New(db *sqlx.DB, group *echo.Group, userSuffix string) *App {
	instance := &App{
		DB:         db,
		Group:      group,
		UserSuffix: userSuffix,
	}

	// swagger:route get /instantlaunches/mappings/defaults instantlaunches listDefaults
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
	instance.Group.GET("/mappings/defaults", instance.GetListDefaults)

	// swagger:route get /instantlaunches/mappings/defaults/latest instantlaunches latestDefaults
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
	instance.Group.GET("/mappings/defaults/latest", instance.GetLatestDefaults)

	// swagger:route put /instantlaunches/mappings/defaults/latest instantlaunches addLatestDefaults
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
	instance.Group.PUT("/mappings/defaults/latest", instance.AddLatestDefaultsHandler)

	// swagger:route post /instantlaunches/mappings/defaults/latest instantlaunches updateLatestDefaults
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
	instance.Group.POST("/mappings/defaults/latest", instance.UpdateLatestDefaultsHandler)

	// swagger:route delete /instantlaunches/mappings/defaults/latest instantlaunches deleteLatestDefaults
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
	instance.Group.DELETE("/mappings/defaults/latest", instance.DeleteLatestDefaultsHandler)

	// swagger:route get /instantlaunches/mappings/defaults/{version} instantlaunches defaultsByVersion
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
	instance.Group.GET("/mappings/defaults/:version", instance.DefaultsByVersionHandler)

	// swagger:route post /instantlaunches/mappings/defaults/{version} instantlaunches updateDefaultsByVersion
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
	instance.Group.POST("/mappings/defaults/:version", instance.UpdateDefaultsByVersionHandler)

	// swagger:route delete /instantlaunches/mappings/defaults/{version} instantlaunches deleteDefaultsByVersion
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
	instance.Group.DELETE("/mappings/defaults/:version", instance.DeleteDefaultsByVersionHandler)

	// swagger:route get /instantlaunches/mappings/{username} instantlaunches getAllUserDefaults
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
	instance.Group.GET("/mappings/:username", instance.AllUserMappingsHandler)

	// swagger:route get /instantlaunches/mappings/{username}/latest instantlaunches getUserDefaults
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
	instance.Group.GET("/mappings/:username/latest", instance.UserMappingHandler)

	// swagger:route put /instantlaunches/mappings/{username}/latest instantlaunches addUserDefaults
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
	instance.Group.PUT("/mappings/:username", instance.AddUserMappingHandler)

	// swagger:route post /instantlaunches/mappings/{username}/latest instantlaunches updateUserDefaults
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
	instance.Group.POST("/mappings/:username/latest", instance.UpdateUserMappingHandler)

	// swagger:route delete /instantlaunches/mappings/{username}/latest instantlaunches deleteUserDefaults
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
	instance.Group.DELETE("/mappings/:username/latest", instance.DeleteUserMappingHandler)

	// swagger:route get /instantlaunches/mappings/{username}/{version} instantlaunches getUserDefaultsByVersion
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
	instance.Group.GET("/mappings/:username/:version", instance.UserMappingsByVersionHandler)

	// swagger:route post /instantlaunches/mappings/{username}/{version} instantlaunches updateUserDefaultsByVersion
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
	instance.Group.POST("/mappings/:username/:version", instance.UpdateUserMappingsByVersionHandler)

	// swagger:route delete /instantlaunches/mappings/{username}/{version} instantlaunches deleteUSerDefaultsByVersion
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
	instance.Group.DELETE("/mappings/:username/:version", instance.DeleteUserMappingsByVersionHandler)

	return instance
}

// swagger:parameters defaultsByVersion
type defaultsByVersionParams struct {
	// in: path
	// required: true
	// minimum: 0
	Version int
}
