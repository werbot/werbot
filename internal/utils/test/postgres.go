package test

import (
	"bytes"
	"context"
	"testing"

	ep "github.com/fergusstrange/embedded-postgres"
	"github.com/pressly/goose/v3"
	"github.com/werbot/werbot/internal/storage/postgres"
)

// DB is ...
type DB struct {
	Server *ep.EmbeddedPostgres
	Conn   *postgres.Connect
}

// CreateDB is ...
func CreateDB(t *testing.T, dirs ...string) (*DB, error) {
	db := new(DB)

	db.Server = ep.NewDatabase(ep.DefaultConfig().
		Version(ep.V15).
		Logger(&bytes.Buffer{}).
		Port(9876))

	if err := db.Server.Start(); err != nil {
		return nil, err
	}

	// connect to postgres
	conn, err := postgres.New(context.Background(), &postgres.PgSQLConfig{
		DSN: "postgres://postgres:postgres@localhost:9876/postgres?sslmode=disable",
	})
	if err != nil {
		return nil, err
	}
	db.Conn = conn

	// migration to postgres
	for _, opt := range dirs {
		if err := goose.Up(db.Conn.Conn, opt); err != nil {
			//log.Fatalf("Migration exited with error: %v", err)
			return nil, err
		}
	}

	return db, nil
}

// Stop is ...
func (d *DB) Stop(t *testing.T) {
	if err := d.Server.Stop(); err != nil {
		t.Error(err)
	}
}
