package instantlaunches

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"

	"github.com/cyverse-de/app-exposer/common"
	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
	"github.com/valyala/fastjson"
)

var log = common.Log

func handleError(err error, statusCode int) error {
	log.Error(err)
	return echo.NewHTTPError(statusCode, err.Error())
}

// InstantLaunchExists returns true if the id passed in exists in the database
func (a *App) InstantLaunchExists(id string) (bool, error) {
	var count int
	err := a.DB.Get(&count, "SELECT COUNT(*) FROM instant_launches WHERE id = $1;", id)
	return count > 0, err
}

func (a *App) listAVUs(c echo.Context) ([]byte, *http.Response, error) {
	log.Debug("in ListMetadataHandler")

	user := c.QueryParam("user")
	if user == "" {
		return nil, nil, echo.NewHTTPError(http.StatusBadRequest, "user is missing")
	}

	attr := c.QueryParam("attribute")
	value := c.QueryParam("value")
	unit := c.QueryParam("unit")

	svc, err := url.Parse(a.MetadataBaseURL)
	if err != nil {
		return nil, nil, handleError(err, http.StatusBadRequest)
	}

	svc.Path = path.Join(svc.Path, "/avus")
	query := svc.Query()
	query.Add("user", user)
	query.Add("target-type", "instant_launch")

	if attr != "" {
		query.Add("attribute", attr)
	}

	if value != "" {
		query.Add("value", value)
	}

	if unit != "" {
		query.Add("unit", unit)
	}
	svc.RawQuery = query.Encode()

	log.Debug(fmt.Sprintf("metadata endpoint: GET %s", svc.String()))

	resp, err := http.Get(svc.String())
	if err != nil {
		return nil, nil, handleError(err, http.StatusInternalServerError)
	}

	log.Debug(fmt.Sprintf("metadata endpoint: %s, status code: %d", svc.String(), resp.StatusCode))

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, resp, handleError(err, http.StatusInternalServerError)
	}

	return body, resp, nil
}

const fullListingQuery = `
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


WHERE il.id = any($1);
`

// FullListMetadataHandler returns a list of instant launches with the quick launches
// embedded, meaning that the submission field is included.
func (a *App) FullListMetadataHandler(c echo.Context) error {
	log.Debug("in FullListMetadataHandler")
	avuBody, _, err := a.listAVUs(c)
	if err != nil {
		return err
	}

	var p fastjson.Parser

	val, err := p.ParseBytes(avuBody)
	if err != nil {
		return err
	}

	avus := val.GetArray("avus")
	if avus == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "no avus found in metadata response")
	}

	var targetIDs []string

	for _, avu := range avus {
		targetIDs = append(targetIDs, string(avu.GetStringBytes("target_id")))
	}

	fullListing := []FullInstantLaunch{}
	if err = a.DB.Select(&fullListing, fullListingQuery, pq.Array(targetIDs)); err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusNotFound, "no instant launches found")
		}
		return err
	}

	return c.JSON(http.StatusOK, fullListing)
}

// ListMetadataHandler lists all of the instant launch metadata
// based on the attributes and values contained in the body.
func (a *App) ListMetadataHandler(c echo.Context) error {
	body, resp, err := a.listAVUs(c)
	if err != nil {
		return err
	}

	return c.Blob(resp.StatusCode, resp.Header.Get(http.CanonicalHeaderKey("content-type")), body)
}

// GetMetadataHandler returns all of the metadata associated with an instant launch.
func (a *App) GetMetadataHandler(c echo.Context) error {
	log.Debug("int GetMetadataHandler")

	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is missing")
	}

	user := c.QueryParam("user")
	if user == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "user is missing")
	}

	exists, err := a.InstantLaunchExists(id)
	if err != nil {
		return handleError(err, http.StatusInternalServerError)
	}

	if !exists {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("instant launch UUID %s not found", id))
	}

	svc, err := url.Parse(a.MetadataBaseURL)
	if err != nil {
		return handleError(err, http.StatusBadRequest)
	}

	svc.Path = path.Join(svc.Path, "/avus", "instant_launch", id)
	query := svc.Query()
	query.Add("user", user)
	svc.RawQuery = query.Encode()

	log.Debug(fmt.Sprintf("metadata endpoint: GET %s", svc.String()))

	resp, err := http.Get(svc.String())
	if err != nil {
		return handleError(err, http.StatusInternalServerError)
	}

	log.Debug(fmt.Sprintf("metadata endpoint: %s, status code: %d", svc.String(), resp.StatusCode))

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return handleError(err, http.StatusInternalServerError)
	}

	return c.Blob(resp.StatusCode, resp.Header.Get(http.CanonicalHeaderKey("content-type")), body)
}

// AddOrUpdateMetadataHandler adds or updates one or more AVUs on an instant
// launch.
func (a *App) AddOrUpdateMetadataHandler(c echo.Context) error {
	log.Debug("in AddOrUpdateMetadataHandler")

	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is missing")
	}

	user := c.QueryParam("user")
	if user == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "user is missing")
	}

	exists, err := a.InstantLaunchExists(id)
	if err != nil {
		return handleError(err, http.StatusInternalServerError)
	}

	if !exists {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("instant launch UUID %s not found", id))
	}

	inBody, err := ioutil.ReadAll(c.Request().Body)
	if err != nil {
		return handleError(err, http.StatusInternalServerError)
	}

	svc, err := url.Parse(a.MetadataBaseURL)
	if err != nil {
		return handleError(err, http.StatusBadRequest)
	}

	svc.Path = path.Join(svc.Path, "/avus", "instant_launch", id)
	query := svc.Query()
	query.Add("user", user)
	svc.RawQuery = query.Encode()

	log.Debug(fmt.Sprintf("metadata endpoint: POST %s", svc.String()))

	resp, err := http.Post(svc.String(), "application/json", bytes.NewReader(inBody))
	if err != nil {
		return handleError(err, http.StatusInternalServerError)
	}

	log.Debug(fmt.Sprintf("metadata endpoint: %s, status code: %d", svc.String(), resp.StatusCode))

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return handleError(err, http.StatusInternalServerError)
	}

	return c.Blob(resp.StatusCode, resp.Header.Get(http.CanonicalHeaderKey("content-type")), body)
}

// SetAllMetadataHandler sets all of the AVUs associated with an instant
// launch to the set contained in the body of the request.
func (a *App) SetAllMetadataHandler(c echo.Context) error {
	log.Debug("in SetAllMetadataHandler")

	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is missing")
	}

	user := c.QueryParam("user")
	if user == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "user is missing")
	}

	exists, err := a.InstantLaunchExists(id)
	if err != nil {
		return handleError(err, http.StatusInternalServerError)
	}

	if !exists {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("instant launch UUID %s not found", id))
	}

	inBody, err := ioutil.ReadAll(c.Request().Body)
	if err != nil {
		return handleError(err, http.StatusInternalServerError)
	}

	svc, err := url.Parse(a.MetadataBaseURL)
	if err != nil {
		return handleError(err, http.StatusBadRequest)
	}

	svc.Path = path.Join(svc.Path, "/avus", "instant_launch", id)
	query := svc.Query()
	query.Add("user", user)
	svc.RawQuery = query.Encode()

	log.Debug(fmt.Sprintf("metadata endpoint: PUT %s", svc.String()))

	req, err := http.NewRequest(http.MethodPut, svc.String(), bytes.NewReader(inBody))
	if err != nil {
		return handleError(err, http.StatusInternalServerError)
	}

	req.Header.Set(http.CanonicalHeaderKey("content-type"), "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return handleError(err, http.StatusInternalServerError)
	}

	log.Debug(fmt.Sprintf("metadata endpoint: %s, status code: %d", svc.String(), resp.StatusCode))

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return handleError(err, http.StatusInternalServerError)
	}

	return c.Blob(resp.StatusCode, resp.Header.Get(http.CanonicalHeaderKey("content-type")), body)
}
