package grpc

func checkUserIDAndProjectID(projectID, userID string) bool {
	var id string
	db.Conn.QueryRow(`SELECT "id" FROM "project" WHERE "id" = $1 AND "owner_id" = $2`, projectID, userID).Scan(&id)
	return id != ""
}
