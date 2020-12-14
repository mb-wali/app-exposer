package instantlaunches

const addInstantLaunchQuery = `
	INSERT INTO instant_launches (quick_launch_id, added_by)
	VALUES ( $1, ( SELECT u.id FROM users u WHERE u.username = $2 ) )
	RETURNING id, quick_launch_id, added_by, added_on;
`

// AddInstantLaunch registers a new instant launch in the database.
func (a *App) AddInstantLaunch(quickLaunchID, username string) (*InstantLaunch, error) {
	newvalues := &InstantLaunch{}
	err := a.DB.QueryRowx(addInstantLaunchQuery, quickLaunchID, username).Scan(&newvalues.ID,
		&newvalues.QuickLaunchID,
		&newvalues.AddedBy,
		&newvalues.AddedOn,
	)
	return newvalues, err
}

const getInstantLaunchQuery = `
	SELECT i.id, i.quick_launch_id, i.added_by, i.added_on
	  FROM instant_launches i
	 WHERE i.id = $1;
`

// GetInstantLaunch returns a stored instant launch by ID.
func (a *App) GetInstantLaunch(id string) (*InstantLaunch, error) {
	il := &InstantLaunch{}
	err := a.DB.QueryRowx(getInstantLaunchQuery, id).Scan(&il.ID,
		&il.QuickLaunchID,
		&il.AddedBy,
		&il.AddedOn,
	)
	return il, err
}

const updateInstantLaunchQuery = `
	UPDATE ONLY instant_launches
			SET quick_launch_id = $1
		  WHERE id = $2;
	  RETURNING id, quick_launch_id, added_by, added_by;
`

// UpdateInstantLaunch updates a stored instant launch with new values.
func (a *App) UpdateInstantLaunch(id, quickLaunchID string) (*InstantLaunch, error) {
	il := &InstantLaunch{}
	err := a.DB.QueryRowx(updateInstantLaunchQuery, quickLaunchID, id).Scan(&il.ID,
		&il.QuickLaunchID,
		&il.AddedBy,
		&il.AddedOn,
	)
	return il, err
}

const deleteInstantLaunchQuery = `
	DELETE FROM instant_launches WHERE id = $1;
`

// DeleteInstantLaunch deletes a stored instant launch.
func (a *App) DeleteInstantLaunch(id string) error {
	_, err := a.DB.Exec(deleteInstantLaunchQuery, id)
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
