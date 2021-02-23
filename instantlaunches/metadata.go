package instantlaunches

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"

	"github.com/labstack/echo/v4"
)

// ListMetadataHandler lists all of the instant launch metadata
// based on the attributes and values contained in the body.
func (a *App) ListMetadataHandler(c echo.Context) error {

	return nil
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
		return err
	}

	svc.Path = path.Join(svc.Path, "/avus", "instant_launch", id)
	query := svc.Query()
	query.Add("user", user)
	svc.RawQuery = query.Encode()

	resp, err := http.Get(svc.String())
	if err != nil {
		return err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
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
		return err
	}

	svc, err := url.Parse(a.MetadataBaseURL)
	if err != nil {
		return err
	}

	svc.Path = path.Join(svc.Path, "/avus", "instant_launch", id)
	query := svc.Query()
	query.Add("user", user)
	svc.RawQuery = query.Encode()

	resp, err := http.Post(svc.String(), "application/json", bytes.NewReader(inBody))
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
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
		return err
	}

	svc, err := url.Parse(a.MetadataBaseURL)
	if err != nil {
		return err
	}

	svc.Path = path.Join(svc.Path, "/avus", "instant_launch", id)
	query := svc.Query()
	query.Add("user", user)
	svc.RawQuery = query.Encode()

	req, err := http.NewRequest(http.MethodPut, svc.String(), bytes.NewReader(inBody))
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return c.Blob(resp.StatusCode, resp.Header.Get(http.CanonicalHeaderKey("content-type")), body)
}
