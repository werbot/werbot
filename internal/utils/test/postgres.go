package test

import (
	"bytes"
	"context"
	"log"
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
func CreateDB(t *testing.T, dirs ...string) *DB {
	db := new(DB)

	db.Server = ep.NewDatabase(ep.DefaultConfig().
		Version(ep.V15).
		Logger(&bytes.Buffer{}).
		Port(9876))

	err := db.Server.Start()
	if err != nil {
		log.Fatalf("Server postgres exited with error: %v", err)
	}

	// connect to postgres
	db.Conn, _ = postgres.New(context.Background(), &postgres.PgSQLConfig{
		DSN: "postgres://postgres:postgres@localhost:9876/postgres?sslmode=disable",
	})

	// migration to postgres
	for _, opt := range dirs {
		if err := goose.Up(db.Conn.Conn, opt); err != nil {
			log.Fatalf("Migration exited with error: %v", err)
		}
	}

	return db
}

// Stop is ...
func (d *DB) Stop(t *testing.T) {
	err := d.Server.Stop()
	if err != nil {
		t.Error(err)
	}
}
