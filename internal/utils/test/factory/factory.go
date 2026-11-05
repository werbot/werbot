package factory

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/werbot/werbot/pkg/crypto"
	"github.com/werbot/werbot/pkg/uuid"
)

// Builder creates database rows for tests.
type Builder struct {
	t  *testing.T
	db *sql.DB
}

// New returns a Builder bound to the test and its database.
func New(t *testing.T, db *sql.DB) *Builder {
	t.Helper()
	return &Builder{t: t, db: db}
}

// Bool returns a pointer to v.
func Bool(v bool) *bool {
	return &v
}

// Time returns a pointer to v.
func Time(v time.Time) *time.Time {
	return &v
}

// uuidOrNew returns v when it is set and a new UUID otherwise.
func uuidOrNew(v string) string {
	if v == "" {
		return uuid.New()
	}
	return v
}

// randomAlias returns a lowercase random string of 8 characters.
func randomAlias() string {
	return strings.ToLower(crypto.NewPassword(8, false))
}

// boolOr returns the value behind v, or def when v is nil.
func boolOr(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}
