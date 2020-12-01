package instantlaunches

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

func SetupApp() (*App, sqlmock.Sqlmock, *echo.Echo, error) {
	mockdb, mock, err := sqlmock.New()
	if err != nil {
		return nil, mock, nil, fmt.Errorf("error connecting to mock database %s", err)
	}

	sqlxMockDB := sqlx.NewDb(mockdb, "sqlmock")
	e := echo.New()
	g := e.Group("/instantlaunches")

	app := New(sqlxMockDB, g)
	return app, mock, e, nil
}

func TestLatestDefaults(t *testing.T) {
	assert := assert.New(t)

	app, mock, _, err := SetupApp()
	if err != nil {
		t.Fatalf("error setting up app: %s", err)
	}
	defer app.DB.Close()

	rows := sqlmock.NewRows([]string{"id", "version", "mapping"}).
		AddRow("0", "0", "{}")

	mock.ExpectQuery(latestDefaultsQuery).WillReturnRows(rows)

	mapping, err := app.LatestDefaults()
	assert.NoError(err, "error from LatestDefaults should be nil")
	assert.Equal("0", mapping.ID, "id should be 0")
	assert.Equal("0", mapping.Version, "version should be 0")
	assert.Equal(0, len(mapping.Mapping), "mapping should be empty")
	assert.NoError(mock.ExpectationsWereMet(), "expectations were not met")
}

func TestGetLatestDefaults(t *testing.T) {
	assert := assert.New(t)

	app, mock, router, err := SetupApp()
	if err != nil {
		t.Fatalf("error setting up app: %s", err)
	}
	defer app.DB.Close()

	expected := map[string]*InstantLaunchSelector{
		"one": &InstantLaunchSelector{
			Pattern: "*",
			Kind:    "glob",
			Default: InstantLaunch{
				ID:            "0",
				QuickLaunchID: "0",
				AddedBy:       "test",
				AddedOn:       "today",
			},
			Compatible: []InstantLaunch{},
		},
	}
	v, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("error unmarshalling expected value: %s", err)
	}

	rows := sqlmock.NewRows([]string{"id", "version", "mapping"}).
		AddRow("0", "0", v)

	mock.ExpectQuery(latestDefaultsQuery).WillReturnRows(rows)

	req, err := http.NewRequest("GET", "http://localhost/instantlaunches/defaults", nil)
	if err != nil {
		t.Fatalf("error creating new http request: %s", err)
	}
	rec := httptest.NewRecorder()

	c := router.NewContext(req, rec)

	err = app.GetLatestDefaults(c)
	if assert.NoError(err, "error from GetLatestDefaults should be nil") {
		assert.Equal(http.StatusOK, rec.Code)

		actual := &DefaultInstantLaunchMapping{}
		err = json.Unmarshal(rec.Body.Bytes(), actual)
		if assert.NoError(err, "should be able to parse body") {
			assert.Equal("0", actual.ID, "id should be 0")
			assert.Equal("0", actual.Version, "version should be 0")
			assert.Equal(expected["one"], actual.Mapping["one"], "mapping should return expected value")
		}
	}
	assert.NoError(mock.ExpectationsWereMet(), "expectations were not met")
}

func TestUpdateLatestDefaults(t *testing.T) {
	assert := assert.New(t)

	app, mock, _, err := SetupApp()
	if err != nil {
		t.Fatalf("error setting up app: %s", err)
	}
	defer app.DB.Close()

	expected := &InstantLaunchMapping{
		"one": &InstantLaunchSelector{
			Pattern: "*",
			Kind:    "glob",
			Default: InstantLaunch{
				ID:            "0",
				QuickLaunchID: "0",
				AddedBy:       "test",
				AddedOn:       "today",
			},
			Compatible: []InstantLaunch{},
		},
	}

	v, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("error unmarshalling expected value: %s", err)
	}

	rows := sqlmock.NewRows([]string{"instant_launches"}).
		AddRow(v)

	mock.ExpectQuery("UPDATE ONLY default_instant_launches").WillReturnRows(rows)

	mapping, err := app.UpdateLatestDefaults(expected)
	assert.NoError(err, "error from UpdateLatestDefaults should be nil")
	assert.True(reflect.DeepEqual(expected, mapping), "mappings should match")
	assert.NoError(mock.ExpectationsWereMet(), "expectations were not met")
}

func TestUpdateLatestDefaultsHandler(t *testing.T) {
	assert := assert.New(t)

	app, mock, router, err := SetupApp()
	if err != nil {
		t.Fatalf("error setting up app: %s", err)
	}
	defer app.DB.Close()

	expected := &InstantLaunchMapping{
		"one": &InstantLaunchSelector{
			Pattern: "*",
			Kind:    "glob",
			Default: InstantLaunch{
				ID:            "0",
				QuickLaunchID: "0",
				AddedBy:       "test",
				AddedOn:       "today",
			},
			Compatible: []InstantLaunch{},
		},
	}

	v, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("error unmarshalling expected value: %s", err)
	}

	rows := sqlmock.NewRows([]string{"instant_launches"}).
		AddRow(v)

	mock.ExpectQuery("UPDATE ONLY default_instant_launches").WillReturnRows(rows)

	req := httptest.NewRequest("PUT", "http://localhost/instantlaunches/defaults", bytes.NewReader(v))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	c := router.NewContext(req, rec)

	err = app.UpdateLatestDefaultsHandler(c)
	assert.NoError(err)
}
