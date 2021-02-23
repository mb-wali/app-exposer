package instantlaunches

import (
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

	svc, err := url.Parse(a.MetadataBaseURL)
	if err != nil {
		return err
	}

	svc.Path = path.Join(svc.Path, "/avus", "quick_launch", id)
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
	return nil
}

// SetAllMetadataHandler sets all of the AVUs associated with an instant
// launch to the set contained in the body of the request.
func (a *App) SetAllMetadataHandler(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is missing")
	}
	return nil
}
