package identity

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/werbot/werbot/internal/identity/dbq"
)

// pgStore implements Store with the sqlc-generated queries.
type pgStore struct{ *dbq.Queries }

// newStore binds the queries to db, which is always a dbx transaction.
func newStore(db dbq.DBTX) Store { return pgStore{dbq.New(db)} }

// isUniqueViolation reports a unique_violation (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	return ok && pgErr.Code == "23505"
}
