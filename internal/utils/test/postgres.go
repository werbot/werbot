package test

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"testing"

	ep "github.com/fergusstrange/embedded-postgres"
	"github.com/pressly/goose/v3"
	"github.com/werbot/werbot/internal/storage/postgres"
)

// PostgresService is ...
type PostgresService struct {
	server *ep.EmbeddedPostgres
	conn   *postgres.Connect
	test   *testing.T
}

// Postgres is ...
func Postgres(t *testing.T, dirs ...string) (*PostgresService, error) {
	service := &PostgresService{
		test: t,
	}

	min := 9500
	max := 9900
	portPG := rand.Intn(max-min) + min

	service.server = ep.NewDatabase(ep.DefaultConfig().
		Version(ep.V15).
		Logger(&bytes.Buffer{}).
		Port(uint32(portPG)))

	if err := service.server.Start(); err != nil {
		return nil, err
	}

	// connect to postgres
	conn, err := postgres.New(context.Background(), &postgres.PgSQLConfig{
		DSN: fmt.Sprintf("postgres://postgres:postgres@localhost:%v/postgres?sslmode=disable", portPG),
	})
	if err != nil {
		return nil, err
	}
	service.conn = conn

	// migration to postgres
	for _, opt := range dirs {
		if err := goose.Up(service.conn.Conn, opt); err != nil {
			return nil, err
		}
	}

	return service, nil
}

// Conn is ...
func (d *PostgresService) Conn() *postgres.Connect {
	return d.conn
}

// Close is ...
func (d *PostgresService) Close() {
	if err := d.server.Stop(); err != nil {
		d.test.Error(err)
	}
}
