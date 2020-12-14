package instantlaunches

import (
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestAddInstantLaunch(t *testing.T) {
	assert := assert.New(t)

	app, mock, _, err := SetupApp()
	if err != nil {
		t.Fatalf("error setting up app: %s", err)
	}
	defer app.DB.Close()

	rows := sqlmock.NewRows([]string{"id", "quick_launch_id", "added_by", "added_on"}).
		AddRow("0", "0", "test@iplantcollaborative.org", "today")

	mock.ExpectQuery("INSERT INTO instant_launches").WillReturnRows(rows)

	actual, err := app.AddInstantLaunch("0", "test@iplantcollaborative.org")
	assert.NoError(err, "error should be nil")
	assert.Equal("0", actual.ID, "id should be 0")
	assert.Equal("0", actual.QuickLaunchID, "quick_launch_id should be 0")
	assert.Equal("test@iplantcollaborative.org", actual.AddedBy, "added_by should be test@iplantcollaborative.org")
	assert.Equal("today", actual.AddedOn, "added_on should be set to 'today'")
	assert.NoError(mock.ExpectationsWereMet(), "expectations were not met")
}

func TestGetInstantLaunch(t *testing.T) {
	assert := assert.New(t)

	app, mock, _, err := SetupApp()
	if err != nil {
		t.Fatalf("error setting up app: %s", err)
	}
	defer app.DB.Close()

	rows := sqlmock.NewRows([]string{"id", "quick_launch_id", "added_by", "added_on"}).
		AddRow("0", "0", "test@iplantcollaborative.org", "today")

	mock.ExpectQuery("SELECT i.id, i.quick_launch_id, i.added_by, i.added_on FROM instant_launches i").
		WillReturnRows(rows)

	actual, err := app.GetInstantLaunch("0")
	assert.NoError(err, "error should be nil")
	assert.Equal("0", actual.ID, "id should be 0")
	assert.Equal("0", actual.QuickLaunchID, "quick_launch_id should be 0")
	assert.Equal("test@iplantcollaborative.org", actual.AddedBy, "added_by should be test@iplantcollaborative.org")
	assert.Equal("today", actual.AddedOn, "added_on should be set to 'today'")
	assert.NoError(mock.ExpectationsWereMet(), "expectations were not met")
}

func TestUpdateInstantLaunch(t *testing.T) {
	assert := assert.New(t)

	app, mock, _, err := SetupApp()
	if err != nil {
		t.Fatalf("error setting up app: %s", err)
	}
	defer app.DB.Close()

	rows := sqlmock.NewRows([]string{"id", "quick_launch_id", "added_by", "added_on"}).
		AddRow("0", "0", "test@iplantcollaborative.org", "today")

	mock.ExpectQuery("UPDATE ONLY instant_launches").
		WillReturnRows(rows)

	actual, err := app.UpdateInstantLaunch("0", "0")
	assert.NoError(err, "error should be nil")
	assert.Equal("0", actual.ID, "id should be 0")
	assert.Equal("0", actual.QuickLaunchID, "quick_launch_id should be 0")
	assert.Equal("test@iplantcollaborative.org", actual.AddedBy, "added_by should be test@iplantcollaborative.org")
	assert.Equal("today", actual.AddedOn, "added_on should be set to 'today'")
	assert.NoError(mock.ExpectationsWereMet(), "expectations were not met")
}

func TestDeleteInstantLaunch(t *testing.T) {
	assert := assert.New(t)

	app, mock, _, err := SetupApp()
	if err != nil {
		t.Fatalf("error setting up app: %s", err)
	}
	defer app.DB.Close()

	mock.ExpectExec("DELETE FROM instant_launches").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = app.DeleteInstantLaunch("0")
	assert.NoError(err, "error should be nil")
	assert.NoError(mock.ExpectationsWereMet(), "expectations were not met")
}

func TestListInstantLaunches(t *testing.T) {
	assert := assert.New(t)

	app, mock, _, err := SetupApp()
	if err != nil {
		t.Fatalf("error setting up app: %s", err)
	}
	defer app.DB.Close()

	expected := []InstantLaunch{
		InstantLaunch{
			ID:            "0",
			QuickLaunchID: "0",
			AddedBy:       "test@iplantcollaborative.org",
			AddedOn:       "today",
		},
		InstantLaunch{
			ID:            "1",
			QuickLaunchID: "1",
			AddedBy:       "test1@iplantcollaborative.org",
			AddedOn:       "yesterday",
		},
	}

	rows := sqlmock.NewRows([]string{"id", "quick_launch_id", "added_by", "added_on"}).
		AddRow(expected[0].ID, expected[0].QuickLaunchID, expected[0].AddedBy, expected[0].AddedOn).
		AddRow(expected[1].ID, expected[1].QuickLaunchID, expected[1].AddedBy, expected[1].AddedOn)

	mock.ExpectQuery("SELECT i.id, i.quick_launch_id, i.added_by, i.added_on FROM instant_launches i").
		WillReturnRows(rows)

	actual, err := app.ListInstantLaunches()
	assert.NoError(err, "error should be nil")
	if assert.True(len(actual) > 0 && len(actual) == len(expected), "length is wrong") {
		for index := range expected {
			assert.True(reflect.DeepEqual(expected[index], actual[index]), "should be equal")
		}
	}
	assert.NoError(mock.ExpectationsWereMet(), "expectations were not met")
}
