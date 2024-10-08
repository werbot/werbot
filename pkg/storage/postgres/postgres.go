package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	// Load jackc package
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Connect is ...
type Connect struct {
	Conn *sql.DB
}

// PgSQLConfig is ...
type PgSQLConfig struct {
	DSN             string
	MaxConn         int
	MaxIdleConn     int
	MaxLifetimeConn int
}

// New creates a new Connect object using the given PgSQLConfig.
func New(ctx context.Context, conf *PgSQLConfig) (*Connect, error) {
	// Create a new DB object using the pgx driver.
	db, err := sql.Open("pgx", conf.DSN)
	if err != nil {
		return nil, fmt.Errorf("could not connect to database: %v", err)
	}

	// Configure the connection pool.
	db.SetMaxOpenConns(conf.MaxConn)
	db.SetMaxIdleConns(conf.MaxIdleConn)
	db.SetConnMaxLifetime(time.Duration(conf.MaxLifetimeConn) * time.Second)

	// Ping the database to ensure connectivity.
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err = db.PingContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not ping database: %v", err)
	}

	// Return the new Connect object.
	return &Connect{
		Conn: db,
	}, nil
}
