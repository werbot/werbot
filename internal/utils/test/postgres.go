package test

import (
	"bytes"
	"context"
	"testing"

	ep "github.com/fergusstrange/embedded-postgres"
	"github.com/pressly/goose/v3"
	"github.com/werbot/werbot/internal/storage/postgres"
)

// PostgresService is ...
type PostgresService struct {
	Server *ep.EmbeddedPostgres
	Conn   *postgres.Connect
	test   *testing.T
}

// Postgres is ...
func Postgres(t *testing.T, dirs ...string) (*PostgresService, error) {
	service := &PostgresService{
		test: t,
	}

	service.Server = ep.NewDatabase(ep.DefaultConfig().
		Version(ep.V15).
		Logger(&bytes.Buffer{}).
		Port(9876))

	if err := service.Server.Start(); err != nil {
		return nil, err
	}

	// connect to postgres
	conn, err := postgres.New(context.Background(), &postgres.PgSQLConfig{
		DSN: "postgres://postgres:postgres@localhost:9876/postgres?sslmode=disable",
	})
	if err != nil {
		return nil, err
	}
	service.Conn = conn

	// migration to postgres
	for _, opt := range dirs {
		if err := goose.Up(service.Conn.Conn, opt); err != nil {
			return nil, err
		}
	}

	return service, nil
}

// Close is ...
func (d *PostgresService) Close() {
	if err := d.Server.Stop(); err != nil {
		d.test.Error(err)
	}
}
