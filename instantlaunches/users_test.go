package instantlaunches

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestUserMapping(t *testing.T) {
	assert := assert.New(t)

	app, mock, _, err := SetupApp()
	if err != nil {
		t.Fatalf("error setting up app: %s", err)
	}
	defer app.DB.Close()

	rows := sqlmock.NewRows([]string{"id", "version", "mapping"}).
		AddRow("0", "0", "{}")

	mock.ExpectQuery("SELECT u.id, u.version, u.instant_launches as mapping FROM user_instant_launches u").
		WithArgs("test").
		WillReturnRows(rows)

	actual, err := app.UserMapping("test")
	assert.NoError(err, "should not error")
	assert.Equal("0", actual.ID, "id should be 0")
	assert.Equal("0", actual.Version, "version should be 0")
	assert.True(
		reflect.DeepEqual(
			&UserInstantLaunchMapping{
				ID:      "0",
				Version: "0",
				Mapping: InstantLaunchMapping{},
			},
			actual,
		),
		"should be equal",
	)
	assert.NoError(mock.ExpectationsWereMet(), "expectataions were not met")
}

func TestUserMappingHandler(t *testing.T) {}

func TestUpdateUserMapping(t *testing.T) {
	assert := assert.New(t)

	app, mock, _, err := SetupApp()
	if err != nil {
		t.Fatalf("error setting up app: %s", err)
	}
	defer app.DB.Close()

	expected := &InstantLaunchMapping{
		"one": {
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
	assert.NoError(err, "no errors expected")

	rows := sqlmock.NewRows([]string{"instant_launches"}).
		AddRow(v)
	mock.ExpectQuery("UPDATE ONLY user_instant_launches AS def").
		WithArgs(v, "test").
		WillReturnRows(rows)

	actual, err := app.UpdateUserMapping("test", expected)
	assert.NoError(err, "no errors expected")
	assert.True(reflect.DeepEqual(expected, actual), "should be equal")
	assert.NoError(mock.ExpectationsWereMet(), "expectataions were not met")
}

func TestUpdateUserMappingHandler(t *testing.T) {}

func TestDeleteUserMapping(t *testing.T) {}

func TestDeleteUserMappingHandler(t *testing.T) {}

func TestAddUserMapping(t *testing.T) {}

func TestAddUserMappingHandler(t *testing.T) {}

func TestAllUserMappings(t *testing.T) {}

func TestAllUserMappingsHandler(t *testing.T) {}

func TestUserMappingsByVersion(t *testing.T) {}

func TestUserMappingsByVersionHandler(t *testing.T) {}

func TestUpdateUserMappingsByVersion(t *testing.T) {}

func TestUpdateUserMappingsByVersionHandler(t *testing.T) {}

func TestDeleteUserMappingsByVersion(t *testing.T) {}

func TestDeleteUserMappingsByVersionHandler(t *testing.T) {}
