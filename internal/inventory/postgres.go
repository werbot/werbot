package inventory

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/werbot/werbot/internal/inventory/dbq"
)

// pgStore implements Store with the sqlc-generated queries.
type pgStore struct{ *dbq.Queries }

// newStore binds the queries to db, which is always a dbx transaction.
func newStore(db dbq.DBTX) Store { return pgStore{dbq.New(db)} }

func pgCode(err error, code string) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	return ok && pgErr.Code == code
}

// isUniqueViolation reports SQLSTATE 23505.
func isUniqueViolation(err error) bool { return pgCode(err, "23505") }

// isForeignKeyViolation reports SQLSTATE 23503.
func isForeignKeyViolation(err error) bool { return pgCode(err, "23503") }
