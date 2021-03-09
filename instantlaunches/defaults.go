package instantlaunches

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

// DefaultInstantLaunchMapping docs
//
// A global default mapping of files to instant launches.
//
// swagger:response defaultMapping
//
//		In: body
type DefaultInstantLaunchMapping struct {
	// Unique identifier.
	//
	// Required: true
	ID string `db:"id" json:"id"`

	// The version of the mapping format.
	//
	// Required: true
	Version string `db:"version" json:"version"` // determines the format.

	// The mapping from files to instant launches.
	//
	// Required: true
	Mapping InstantLaunchMapping `db:"mapping" json:"mapping"`
}

// LatestDefaultsHandler is the echo handler for the http API that returns the
// default mapping of instant launches to file patterns.
func (a *App) LatestDefaultsHandler(c echo.Context) error {
	defaults, err := a.LatestDefaults()
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}
		return err
	}
	return c.JSON(http.StatusOK, defaults)
}

// UpdateLatestDefaultsHandler is the echo handler for the HTTP API that updates
// the latest defaults mapping.
func (a *App) UpdateLatestDefaultsHandler(c echo.Context) error {
	newdefaults, err := InstantLaunchMappingFromJSON(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot parse JSON")
	}
	updated, err := a.UpdateLatestDefaults(newdefaults)
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}
		return err
	}
	return c.JSON(http.StatusOK, updated)
}

// DeleteLatestDefaultsHandler is the echo handler for the HTTP API that allows
// the caller to delete the latest default mappings from the database.
func (a *App) DeleteLatestDefaultsHandler(c echo.Context) error {
	return a.DeleteLatestDefaults()
}

// AddLatestDefaultsHandler is the echo handler for the HTTP API that allows the
// caller to add a new version of the default instant launch mapping to the db.
func (a *App) AddLatestDefaultsHandler(c echo.Context) error {
	addedBy := c.QueryParam("username")
	if addedBy == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing username in query parameters")
	}

	if !strings.HasSuffix(addedBy, a.UserSuffix) {
		addedBy = fmt.Sprintf("%s%s", addedBy, a.UserSuffix)
	}

	update, err := InstantLaunchMappingFromJSON(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot parse JSON")
	}

	newentry, err := a.AddLatestDefaults(update, addedBy)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, newentry)
}

// DefaultsByVersionHandler is the echo handler for the http API that returns the defaults
// stored for the provided format version.
func (a *App) DefaultsByVersionHandler(c echo.Context) error {
	version, err := strconv.ParseInt(c.Param("version"), 10, 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot process version")
	}

	m, err := a.DefaultsByVersion(int(version))
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}
		return err
	}

	return c.JSON(http.StatusOK, m)
}

// UpdateDefaultsByVersionHandler is the echo handler for the HTTP API that
// updates the default mapping for a specific version.
func (a *App) UpdateDefaultsByVersionHandler(c echo.Context) error {
	// I'm not sure why, but this stuff seems to break echo's c.Bind() function
	// so we handle the unmarshalling without it here.
	newvalue, err := InstantLaunchMappingFromJSON(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot parse JSON")
	}

	version, err := strconv.ParseInt(c.Param("version"), 10, 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot process version")
	}

	updated, err := a.UpdateDefaultsByVersion(newvalue, int(version))
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}
		return err
	}

	return c.JSON(http.StatusOK, updated)
}

// DeleteDefaultsByVersionHandler is an echo handler for the HTTP API that allows the
// caller to delete a default instant launch mapping from the database based on its
// version.
func (a *App) DeleteDefaultsByVersionHandler(c echo.Context) error {
	version, err := strconv.ParseInt(c.Param("version"), 10, 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot process version")
	}
	return a.DeleteDefaultsByVersion(int(version))
}

// A ListAllDefaultsResponse is the response body for listing all of the default mappings.
//
// swagger:response listAllDefaults
//
//		In: Body
type ListAllDefaultsResponse struct {
	// The defaults being listed.
	Defaults []DefaultInstantLaunchMapping `json:"defaults"`
}

// ListDefaultsHandler is the echo handler for the http API that returns a list of
// all defaults listed in the database, regardless of version.
func (a *App) ListDefaultsHandler(c echo.Context) error {
	m, err := a.ListAllDefaults()
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}
		return err
	}
	return c.JSON(http.StatusOK, m)
}
