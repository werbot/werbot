package pg

import (
	"slices"
	"testing"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// TestFunctionsExistInPG18 checks every name of the function table against
// pg_catalog of the test database (PostgreSQL 18).
func TestFunctionsExistInPG18(t *testing.T) {
	pool, _ := dbtest.New(t)
	names := []string{setConfig}
	for name := range functions {
		names = append(names, name)
	}
	slices.Sort(names)
	rows, err := pool.Query(t.Context(), `SELECT n FROM unnest($1::text[]) AS n
		WHERE NOT EXISTS (SELECT FROM pg_proc WHERE proname = n AND pronamespace = 'pg_catalog'::regnamespace)
		ORDER BY n`, names)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		t.Errorf("function %s is not in pg_catalog", name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}
