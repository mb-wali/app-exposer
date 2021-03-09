package instantlaunches

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

const addInstantLaunchQuery = `
	INSERT INTO instant_launches (quick_launch_id, added_by)
	VALUES ( $1, ( SELECT u.id FROM users u WHERE u.username = $2 ) )
	RETURNING id, quick_launch_id, added_by, added_on;
`

// AddInstantLaunch registers a new instant launch in the database.
func (a *App) AddInstantLaunch(quickLaunchID, username string) (*InstantLaunch, error) {
	newvalues := &InstantLaunch{}
	err := a.DB.QueryRowx(addInstantLaunchQuery, quickLaunchID, username).StructScan(newvalues)
	return newvalues, err
}

// AddInstantLaunchHandler is the HTTP handler for adding a new instant launch.
func (a *App) AddInstantLaunchHandler(c echo.Context) error {
	il, err := NewInstantLaunchFromJSON(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot parse JSON")
	}

	if il.AddedBy == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "username was not set")
	}

	if !strings.HasSuffix(il.AddedBy, a.UserSuffix) {
		il.AddedBy = fmt.Sprintf("%s%s", il.AddedBy, a.UserSuffix)
	}

	newil, err := a.AddInstantLaunch(il.QuickLaunchID, il.AddedBy)
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}
		return err
	}

	return c.JSON(http.StatusOK, newil)
}

const getInstantLaunchQuery = `
	SELECT i.id, i.quick_launch_id, i.added_by, i.added_on
	FROM instant_launches i
	WHERE i.id = $1;
`

// GetInstantLaunch returns a stored instant launch by ID.
func (a *App) GetInstantLaunch(id string) (*InstantLaunch, error) {
	il := &InstantLaunch{}
	err := a.DB.QueryRowx(getInstantLaunchQuery, id).StructScan(il)
	return il, err
}

// GetInstantLaunchHandler is the HTTP handler for getting a specific Instant Launch
// by its UUID.
func (a *App) GetInstantLaunchHandler(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is missing")
	}

	il, err := a.GetInstantLaunch(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}
		return err
	}

	return c.JSON(http.StatusOK, il)

}

const fullInstantLaunchQuery = `
SELECT
	il.id,
	ilu.username AS added_by,
	il.added_on,
	il.quick_launch_id,
	ql.name AS ql_name,
	ql.description AS ql_description,
	qlu.username AS ql_creator,
	sub.submission AS submission,
	ql.app_id,
	ql.is_public,
	a.name AS app_name,
	a.description AS app_description,
	a.deleted AS app_deleted,
	a.disabled AS app_disabled,
	iu.username as integrator


FROM instant_launches il
	JOIN quick_launches ql ON il.quick_launch_id = ql.id
	JOIN submissions sub ON ql.submission_id = sub.id
	JOIN apps a ON ql.app_id = a.id
	JOIN integration_data integ ON a.integration_data_id = integ.id
	JOIN users iu ON integ.user_id = iu.id
	JOIN users qlu ON ql.creator = qlu.id
	JOIN users ilu ON il.added_by = ilu.id


WHERE il.id = $1;
`

// FullInstantLaunch returns an instant launch from the database that
// includes quick launch, app, and submission information.
func (a *App) FullInstantLaunch(id string) (*FullInstantLaunch, error) {
	fil := &FullInstantLaunch{}
	err := a.DB.QueryRowx(fullInstantLaunchQuery, id).StructScan(fil)
	return fil, err
}

// FullInstantLaunchHandler is the HTTP handler for getting a full description of
// an instant launch, including its quick launch, submission, and basic app info.
func (a *App) FullInstantLaunchHandler(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is missing")
	}

	il, err := a.FullInstantLaunch(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}
		return err
	}

	return c.JSON(http.StatusOK, il)
}

const updateInstantLaunchQuery = `
	UPDATE ONLY instant_launches
	SET quick_launch_id = $1
	WHERE id = $2
	RETURNING id, quick_launch_id, added_by, added_by;
`

// UpdateInstantLaunch updates a stored instant launch with new values.
func (a *App) UpdateInstantLaunch(id, quickLaunchID string) (*InstantLaunch, error) {
	il := &InstantLaunch{}
	err := a.DB.QueryRowx(updateInstantLaunchQuery, quickLaunchID, id).StructScan(il)
	return il, err
}

// UpdateInstantLaunchHandler is the HTTP handler for updating an instant launch.
func (a *App) UpdateInstantLaunchHandler(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusNotFound, "id is missing")
	}

	updated, err := NewInstantLaunchFromJSON(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot parse JSON")
	}

	newvalue, err := a.UpdateInstantLaunch(id, updated.QuickLaunchID)
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}
		return err
	}

	return c.JSON(http.StatusOK, newvalue)
}

const deleteInstantLaunchQuery = `
	DELETE FROM instant_launches WHERE id = $1;
`

// DeleteInstantLaunch deletes a stored instant launch.
func (a *App) DeleteInstantLaunch(id string) error {
	_, err := a.DB.Exec(deleteInstantLaunchQuery, id)
	return err
}

// DeleteInstantLaunchHandler is the HTTP handler for deleting an Instant Launch
// based on its UUID.
func (a *App) DeleteInstantLaunchHandler(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusNotFound, "id is missing")
	}

	err := a.DeleteInstantLaunch(id)
	return err

}

const listInstantLaunchesQuery = `
	SELECT i.id, i.quick_launch_id, i.added_by, i.added_on
	FROM instant_launches i;
`

// ListInstantLaunches lists all registered instant launches.
func (a *App) ListInstantLaunches() ([]InstantLaunch, error) {
	all := []InstantLaunch{}
	err := a.DB.Select(&all, listInstantLaunchesQuery)
	return all, err
}

// ListInstantLaunchesHandler is the HTTP handler for listing all of the
// registered Instant Launches.
func (a *App) ListInstantLaunchesHandler(c echo.Context) error {
	list, err := a.ListInstantLaunches()
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}
		return err
	}

	return c.JSON(http.StatusOK, list)
}
