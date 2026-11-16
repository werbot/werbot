package identity

import "github.com/werbot/werbot/internal/identity/dbq"

// pgStore implements Store with the sqlc-generated queries.
type pgStore struct{ *dbq.Queries }

// newStore binds the queries to db, which is always a dbx transaction.
func newStore(db dbq.DBTX) Store { return pgStore{dbq.New(db)} }
