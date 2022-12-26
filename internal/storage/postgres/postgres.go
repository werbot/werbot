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

// New is ...
func New(ctx context.Context, conf *PgSQLConfig) (*Connect, error) {
	db, err := sql.Open("pgx", conf.DSN)
	if err != nil {
		return nil, fmt.Errorf("error, not connected to database, %w", err)
	}

	db.SetMaxOpenConns(conf.MaxConn)
	db.SetMaxIdleConns(conf.MaxIdleConn)
	db.SetConnMaxLifetime(time.Duration(conf.MaxLifetimeConn) * time.Second)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("error, not sent ping to database, %w", err)
	}

	return &Connect{
		Conn: db,
	}, nil
}
