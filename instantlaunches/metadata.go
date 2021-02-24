package instantlaunches

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"

	"github.com/cyverse-de/app-exposer/common"
	"github.com/labstack/echo/v4"
)

var log = common.Log

func handleError(err error, statusCode int) error {
	log.Error(err)
	return echo.NewHTTPError(statusCode, err.Error())
}

// ListMetadataHandler lists all of the instant launch metadata
// based on the attributes and values contained in the body.
func (a *App) ListMetadataHandler(c echo.Context) error {
	user := c.QueryParam("user")
	if user == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "user is missing")
	}

	attr := c.QueryParam("attribute")
	value := c.QueryParam("value")
	unit := c.QueryParam("unit")

	svc, err := url.Parse(a.MetadataBaseURL)
	if err != nil {
		return handleError(err, http.StatusBadRequest)
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

	log.Debug(fmt.Sprintf("metadata endpoint: %s", svc.String()))

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

// GetMetadataHandler returns all of the metadata associated with an instant launch.
func (a *App) GetMetadataHandler(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is missing")
	}

	user := c.QueryParam("user")
	if user == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "user is missing")
	}

	svc, err := url.Parse(a.MetadataBaseURL)
	if err != nil {
		return handleError(err, http.StatusBadRequest)
	}

	svc.Path = path.Join(svc.Path, "/avus", "instant_launch", id)
	query := svc.Query()
	query.Add("user", user)
	svc.RawQuery = query.Encode()

	resp, err := http.Get(svc.String())
	if err != nil {
		return handleError(err, http.StatusInternalServerError)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return handleError(err, http.StatusInternalServerError)
	}

	return c.Blob(resp.StatusCode, resp.Header.Get(http.CanonicalHeaderKey("content-type")), body)
}

// AddOrUpdateMetadataHandler adds or updates one or more AVUs on an instant
// launch.
func (a *App) AddOrUpdateMetadataHandler(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is missing")
	}

	user := c.QueryParam("user")
	if user == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "user is missing")
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

	resp, err := http.Post(svc.String(), "application/json", bytes.NewReader(inBody))
	if err != nil {
		return handleError(err, http.StatusInternalServerError)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return handleError(err, http.StatusInternalServerError)
	}

	return c.Blob(resp.StatusCode, resp.Header.Get(http.CanonicalHeaderKey("content-type")), body)
}

// SetAllMetadataHandler sets all of the AVUs associated with an instant
// launch to the set contained in the body of the request.
func (a *App) SetAllMetadataHandler(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is missing")
	}

	user := c.QueryParam("user")
	if user == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "user is missing")
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

	req, err := http.NewRequest(http.MethodPut, svc.String(), bytes.NewReader(inBody))
	if err != nil {
		return handleError(err, http.StatusInternalServerError)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return handleError(err, http.StatusInternalServerError)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return handleError(err, http.StatusInternalServerError)
	}

	return c.Blob(resp.StatusCode, resp.Header.Get(http.CanonicalHeaderKey("content-type")), body)
}
