package postgres

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	// Load jackc package
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Connect is ...
type Connect struct {
	Conn *sqlx.DB
}

// PgSQLConfig is ...
type PgSQLConfig struct {
	DSN             string
	MaxConn         int
	MaxIdleConn     int
	MaxLifetimeConn int
}

// New is ...
func New(conf *PgSQLConfig) (*Connect, error) {
	db, err := sqlx.Connect("pgx", conf.DSN)
	if err != nil {
		return nil, fmt.Errorf("error, not connected to database, %w", err)
	}

	db.SetMaxOpenConns(conf.MaxConn)
	db.SetMaxIdleConns(conf.MaxIdleConn)
	db.SetConnMaxLifetime(time.Duration(conf.MaxLifetimeConn) * time.Second)

	if err := db.Ping(); err != nil {
		defer db.Close()
		return nil, fmt.Errorf("error, not sent ping to database, %w", err)
	}

	return &Connect{
		Conn: db,
	}, nil
}
