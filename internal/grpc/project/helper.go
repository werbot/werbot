package project

import (
  "context"

  "github.com/werbot/werbot/internal/storage/postgres"
)

// IsOwnerProject checks if the user with the given ID owns the project with the given ID.
// It queries the database using the provided db connection and returns true if a project with the given IDs exists and has the user as its owner, false otherwise.
func IsOwnerProject(ctx context.Context, db *postgres.Connect, projectID, userID string) bool {
  var count int
  db.Conn.QueryRowContext(ctx, `
    SELECT
      COUNT(*)
    FROM
      "project"
    WHERE
      "id" = $1
      AND "owner_id" = $2
  `,
    projectID,
    userID,
  ).Scan(&count)
  return count > 0
}
