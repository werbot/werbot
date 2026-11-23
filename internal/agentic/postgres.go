package agentic

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/werbot/werbot/internal/agentic/dbq"
)

// newStore binds the queries to db: a dbx transaction or a dbx.Read.
func newStore(db dbq.DBTX) Store { return dbq.New(db) }

// isForeignKeyViolation reports SQLSTATE 23503.
func isForeignKeyViolation(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	return ok && pgErr.Code == "23503"
}
