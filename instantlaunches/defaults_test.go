package instantlaunches

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

func SetupApp() (*App, sqlmock.Sqlmock, error) {
	mockdb, mock, err := sqlmock.New()
	if err != nil {
		return nil, mock, fmt.Errorf("error connecting to mock database %s", err)
	}

	sqlxMockDB := sqlx.NewDb(mockdb, "sqlmock")
	e := echo.New()
	g := e.Group("/instantlaunches")

	app := New(sqlxMockDB, g)
	return app, mock, nil
}

func TestLatestDefaults(t *testing.T) {
	app, mock, err := SetupApp()
	if err != nil {
		t.Fatalf("error setting up app: %s", err)
	}
	defer app.DB.Close()

	rows := sqlmock.NewRows([]string{"id", "version", "mapping"}).
		AddRow("0", "0", "{}")

	mock.ExpectQuery(latestDefaultsQuery).WillReturnRows(rows)

	mapping, err := app.LatestDefaults()
	if err != nil {
		t.Errorf("error getting latest defaults: %s", err)
	}

	assert := assert.New(t)
	assert.Equal("0", mapping.ID, "id should be 0")
	assert.Equal("0", mapping.Version, "version should be 0")
	assert.Equal(0, len(mapping.Mapping), "mapping should be empty")
	assert.NoError(mock.ExpectationsWereMet(), "expectations were not met")
}
