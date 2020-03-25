package apps

import "database/sql"

// Apps provides an API for accessing information about apps.
type Apps struct {
	DB *sql.DB
}

// NewApps allocates a new *Apps instance.
func NewApps(db *sql.DB) *Apps {
	return &Apps{
		DB: db,
	}
}

const analysisIDByExternalIDQuery = `
	SELECT j.id
	  FROM jobs j
	  JOIN job_steps s ON s.job_id = j.id
	 WHERE s.external_id = $1
`

// GetAnalysisIDByExternalID returns the analysis ID based on the external ID
// passed in.
func (a *Apps) GetAnalysisIDByExternalID(externalID string) (string, error) {
	var analysisID string
	err := a.DB.QueryRow(analysisIDByExternalIDQuery, externalID).Scan(&analysisID)
	if err != nil {
		return "", err
	}
	return analysisID, nil
}

const analysisIDBySubdomainQuery = `
	SELECT j.id
	  FROM jobs j
	 WHERE j.subdomain = $1
`

// GetAnalysisIDBySubdomain returns the analysis ID based on the subdomain
// generated for it.
func (a *Apps) GetAnalysisIDBySubdomain(subdomain string) (string, error) {
	var analysisID string
	err := a.DB.QueryRow(analysisIDBySubdomainQuery, subdomain).Scan(&analysisID)
	if err != nil {
		return "", err
	}
	return analysisID, nil
}

const getUserIPQuery = `
	SELECT l.ip_addpress
	  FROM logins l
	  JOIN users u on l.user_id = u.id
	 WHERE u.id = $1
  ORDER BY l.login_time DESC
     LIMIT 1
`

// GetUserIP returns the latest login ip address for the given user ID.
func (a *Apps) GetUserIP(userID string) (string, error) {
	var ipAddr string
	err := a.DB.QueryRow(getUserIPQuery, userID).Scan(&ipAddr)
	if err != nil {
		return "", err
	}
	return ipAddr, nil
}
