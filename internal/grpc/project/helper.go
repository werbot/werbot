package project

import "github.com/werbot/werbot/internal/storage/postgres"

// IsOwnerProject is ...
func IsOwnerProject(db *postgres.Connect, projectID, userID string) bool {
	var id string
	db.Conn.QueryRow(`SELECT "id" FROM "project" WHERE "id" = $1 AND "owner_id" = $2`,
		projectID,
		userID,
	).Scan(&id)
	return id != ""
}
