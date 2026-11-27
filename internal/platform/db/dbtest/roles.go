package dbtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/jackc/pgx/v5"
)

// Role creates, as the cluster's superuser (roles are cluster-wide), a login role with a random name (r_<hex>) and password and
// the extra attributes attrs (for example "NOINHERIT"). t.Cleanup drops what
// the role owns or was granted in every database, then the role.
func Role(t testing.TB, attrs string) (name, password string) {
	t.Helper()
	name, password = "r_"+randHex(8), randHex(16)
	Exec(t, "CREATE ROLE "+name+" LOGIN PASSWORD '"+password+"' "+attrs)
	t.Cleanup(func() { dropRole(t, name) })
	return name, password
}

// Exec runs each statement as the superuser in database postgres.
func Exec(t testing.TB, sql ...string) {
	t.Helper()
	execIn(t, "postgres", sql...)
}

// execIn runs each statement as the superuser in database db.
func execIn(t testing.TB, db string, sql ...string) {
	t.Helper()
	cfg, err := pgx.ParseConfig(cluster.URL())
	if err != nil {
		t.Fatalf("dbtest: admin config: %v", err)
	}
	cfg.Database = db
	ctx := context.Background()
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("dbtest: admin login to %s: %v", db, err)
	}
	defer conn.Close(ctx)
	for _, s := range sql {
		if _, err := conn.Exec(ctx, s); err != nil {
			t.Fatalf("dbtest: exec in %s: %v", db, err)
		}
	}
}

// dropRole drops role name after DROP OWNED BY in each database that holds
// one of its objects or privileges.
func dropRole(t testing.TB, name string) {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, cluster.URL())
	if err != nil {
		t.Errorf("dbtest: admin login: %v", err)
		return
	}
	rows, err := conn.Query(ctx, `SELECT DISTINCT d.datname FROM pg_shdepend s JOIN pg_database d ON d.oid = s.dbid
		WHERE s.refclassid = 'pg_authid'::regclass AND s.refobjid = (SELECT oid FROM pg_roles WHERE rolname = $1)`, name)
	var dbs []string
	if err == nil {
		dbs, err = pgx.CollectRows(rows, pgx.RowTo[string])
	}
	_ = conn.Close(ctx)
	if err != nil {
		t.Errorf("dbtest: databases of role %s: %v", name, err)
		return
	}
	for _, db := range dbs {
		execIn(t, db, "DROP OWNED BY "+name)
	}
	Exec(t, "DROP ROLE IF EXISTS "+name)
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b) // never fails
	return hex.EncodeToString(b)
}
