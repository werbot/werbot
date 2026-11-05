package test

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/peterldowns/pgtestdb"
	"github.com/pressly/goose/v3"

	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/utils/fsutil"
)

// PostgresService is ...
type PostgresService struct {
	conn *postgres.Connect
}

var gooseLock sync.Mutex

// GooseMigrator is a migrator for pgtestdb.
type GooseMigrator struct {
	dirs []string
	test *testing.T
}

// ServerPostgres is ...
func ServerPostgres(t *testing.T, dirs ...string) *PostgresService {
	db := pgtestdb.New(t, pgtestdb.Config{
		DriverName: "pgx",
		Host:       "localhost",
		User:       "postgres",
		Password:   "password",
		Port:       "5433",
		Options:    "sslmode=disable",
	}, &GooseMigrator{
		dirs: dirs,
		test: t,
	})

	return &PostgresService{
		conn: &postgres.Connect{
			Conn: db,
		},
	}
}

// Conn is ...
func (d *PostgresService) Conn() *postgres.Connect {
	return d.conn
}

// Hash returns the md5 hash of the schema file.
func (gm *GooseMigrator) Hash() (string, error) {
	var hashMap []string
	for _, dir := range gm.dirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() {
				hash, err := fsutil.HashFile(path)
				if err != nil {
					gm.test.Errorf("Failed to compute MD5 for %s: %v", path, err)
				} else {
					hashMap = append(hashMap, hash)
				}
			}
			return nil
		})
		if err != nil {
			gm.test.Fatalf("Error walking the path %q: %v\n", dir, err)
		}
	}

	concatenatedString := strings.Join(hashMap, "")
	hash := md5.New()
	hash.Write([]byte(concatenatedString))
	hashInBytes := hash.Sum(nil)

	return hex.EncodeToString(hashInBytes), nil
}

// Migrate runs migrate.Up() to migrate the template database.
func (gm *GooseMigrator) Migrate(_ context.Context, db *sql.DB, _ pgtestdb.Config) error {
	gooseLock.Lock()
	defer gooseLock.Unlock()

	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	// Every directory gets its own version table so the core and add-on
	// migration histories stay independent. Tests no longer load data fixtures
	// through this path.
	for i, dir := range gm.dirs {
		goose.SetTableName(fmt.Sprintf("goose_db_version_%d", i))
		if err := goose.Up(db, dir); err != nil {
			return fmt.Errorf("%s: %w", dir, err)
		}
	}
	return nil
}

// Prepare is a no-op method.
func (*GooseMigrator) Prepare(_ context.Context, _ *sql.DB, _ pgtestdb.Config) error {
	return nil
}

// Verify is a no-op method.
func (*GooseMigrator) Verify(_ context.Context, _ *sql.DB, _ pgtestdb.Config) error {
	return nil
}
