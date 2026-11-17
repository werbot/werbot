package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDatabaseHosts(t *testing.T) {
	for url, want := range map[string][]string{
		"postgres://u:p@db.internal:5432/werbot":           {"db.internal"},
		"postgres://u:p@db1:5432,db2:5433,db1:5434/werbot": {"db1", "db2"},
		"host=10.0.0.9 port=5432 user=u dbname=werbot":     {"10.0.0.9"},
		"host=/var/run/postgresql user=u dbname=werbot":    nil,
		"postgres://u:p@[::1]:5432/werbot?sslmode=disable": {"::1"},
		"postgres://u:p@db:notaport/werbot":                nil, // unparsable
	} {
		assert.Equal(t, want, databaseHosts(url), url)
	}
}
