package instantlaunches

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

// A DefaultInstantLaunchMapping is a global default mapping of files to instant launches.
//
// swagger:response defaultMapping
//
//		In: body
type DefaultInstantLaunchMapping struct {
	// Unique identifier.
	//
	// Required: true
	ID string `db:"id"`

	// The version of the mapping format.
	//
	// Required: true
	Version string `db:"version"` // determines the format.

	// The mapping from files to instant launches.
	//
	// Required: true
	Mapping InstantLaunchMapping `db:"mapping"`
}

const latestDefaultsQuery = `
    SELECT def.id,
           def.version,
           def.instant_launches AS mapping
      FROM default_instant_launches def
  ORDER BY def.version DESC
     LIMIT 1;
`

const updateLatestDefaultsQuery = `
    UPDATE ONLY default_instant_launches AS def
            SET def.instant_launches = jsonb_object(?)
          WHERE def.version = (
              SELECT max(version)
                FROM default_instant_launches
          )
          RETURNING def.instant_launches;
`

const deleteLatestDefaultsQuery = `
	DELETE FROM ONLY default_instant_launches AS def
	WHERE def.version = (
		SELECT max(version)
		FROM default_instant_launches
	);
`

const createLatestDefaultsQuery = `
	INSERT INTO default_instant_launches (instant_launches)
	VALUES ( ? )
	RETURNING instant_launches;
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

// UpdateLatestDefaults sets a new value for the latest version of the defaults.
func (a *App) UpdateLatestDefaults(newjson *InstantLaunchMapping) (*InstantLaunchMapping, error) {
	marshalled, err := json.Marshal(newjson)
	if err != nil {
		return nil, err
	}
	retval := &InstantLaunchMapping{}
	err = a.DB.QueryRowx(updateLatestDefaultsQuery, marshalled).Scan(retval)
	return retval, err
}

// UpdateLatestDefaultsHandler is the echo handler for the HTTP API that updates
// the latest defaults mapping.
func (a *App) UpdateLatestDefaultsHandler(c echo.Context) error {
	newdefaults := &InstantLaunchMapping{}
	err := c.Bind(newdefaults)
	if err != nil {
		return err
	}
	updated, err := a.UpdateLatestDefaults(newdefaults)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, updated)
}

// DeleteLatestDefaults removes the latest default mappings from the database.
func (a *App) DeleteLatestDefaults() error {
	_, err := a.DB.Exec(deleteLatestDefaultsQuery)
	return err
}

// DeleteLatestDefaultsHandler is the echo handler for the HTTP API that allows
// the caller to delete the latest default mappings from the database.
func (a *App) DeleteLatestDefaultsHandler(c echo.Context) error {
	return a.DeleteLatestDefaults()
}

// AddLatestDefaults adds a new version of the default instant launch mappings.
func (a *App) AddLatestDefaults(update *InstantLaunchMapping) (*InstantLaunchMapping, error) {
	marshalled, err := json.Marshal(update)
	if err != nil {
		return nil, err
	}
	newvalue := &InstantLaunchMapping{}
	err = a.DB.QueryRowx(createLatestDefaultsQuery, marshalled).Scan(newvalue)
	return newvalue, err
}

// AddLatestDefaultsHandler is the echo handler for the HTTP API that allows the
// caller to add a new version of the default instant launch mapping to the db.
func (a *App) AddLatestDefaultsHandler(c echo.Context) error {
	update := &InstantLaunchMapping{}
	err := c.Bind(update)
	if err != nil {
		return err
	}
	newentry, err := a.AddLatestDefaults(update)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, newentry)
}

const defaultsByVersionQuery = `
    SELECT def.id,
           def.version,
           def.instant_launches as mapping
      FROM default_instant_launches def
     WHERE def.version = ?;
`

const updateDefaultsByVersionQuery = `
    UPDATE ONLY default_instant_launches AS def
            SET def.instant_launches = jsonb_object(?)
          WHERE def.version = ?
      RETURNING def.instant_launches;
`

const deleteDefaultsByVersionQuery = `
	DELETE FROM ONLY default_instant_launches as def
	WHERE def.version = ?;
`

// DefaultsByVersion returns a specific version of the default instant launches.
func (a *App) DefaultsByVersion(version int) (*DefaultInstantLaunchMapping, error) {
	m := &DefaultInstantLaunchMapping{}
	err := a.DB.Get(m, defaultsByVersionQuery, version)
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

// UpdateDefaultsByVersion updates the default mapping for a specific version.
func (a *App) UpdateDefaultsByVersion(newjson *InstantLaunchMapping, version int) (*InstantLaunchMapping, error) {
	marshalled, err := json.Marshal(newjson)
	if err != nil {
		return nil, err
	}
	updated := &InstantLaunchMapping{}
	err = a.DB.QueryRowx(updateDefaultsByVersionQuery, marshalled, version).Scan(updated)
	return updated, err
}

// UpdateDefaultsByVersionHandler is the echo handler for the HTTP API that
// updates the default mapping for a specific version.
func (a *App) UpdateDefaultsByVersionHandler(c echo.Context) error {
	newvalue := &InstantLaunchMapping{}
	err := c.Bind(newvalue)
	if err != nil {
		return err
	}

	version, err := strconv.ParseInt(c.Param("version"), 10, 0)
	if err != nil {
		return err
	}

	updated, err := a.UpdateDefaultsByVersion(newvalue, int(version))
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, updated)
}

// DeleteDefaultsByVersion removes a default instant launch mapping from the database
// based on its version.
func (a *App) DeleteDefaultsByVersion(version int) error {
	_, err := a.DB.Exec(deleteDefaultsByVersionQuery, version)
	return err
}

// DeleteDefaultsByVersionHandler is an echo handler for the HTTP API that allows the
// caller to delete a default instant launch mapping from the database based on its
// version.
func (a *App) DeleteDefaultsByVersionHandler(c echo.Context) error {
	version, err := strconv.ParseInt(c.Param("version"), 10, 0)
	if err != nil {
		return err
	}
	return a.DeleteDefaultsByVersion(int(version))
}

const listAllDefaultsQuery = `
    SELECT def.id,
           def.version,
           def.instant_launches as mapping
      FROM default_instant_launches def;
`

// A ListAllDefaultsResponse is the response body for listing all of the default mappings.
//
// swagger:response listAllDefaults
//
//		In: Body
type ListAllDefaultsResponse struct {
	// The defaults being listed.
	Defaults []DefaultInstantLaunchMapping `json:"defaults"`
}

// ListAllDefaults returns a list of all of the default instant launches, including their version.
func (a *App) ListAllDefaults() (ListAllDefaultsResponse, error) {
	m := ListAllDefaultsResponse{Defaults: []DefaultInstantLaunchMapping{}}
	err := a.DB.Select(&m.Defaults, listAllDefaultsQuery)
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
