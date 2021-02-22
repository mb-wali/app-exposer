package instantlaunches

import "github.com/labstack/echo/v4"

// ListMetadataHandler lists all of the instant launch metadata
// based on the attributes and values contained in the body.
func (a *App) ListMetadataHandler(c echo.Context) error {
	return nil
}

// GetMetadataHandler returns all of the metadata associated with an instant launch.
func (a *App) GetMetadataHandler(c echo.Context) error {
	return nil
}

// AddOrUpdateMetadataHandler adds or updates one or more AVUs on an instant
// launch.
func (a *App) AddOrUpdateMetadataHandler(c echo.Context) error {
	return nil
}

// SetAllMetadataHandler sets all of the AVUs associated with an instant
// launch to the set contained in the body of the request.
func (a *App) SetAllMetadataHandler(c echo.Context) error {
	return nil
}
