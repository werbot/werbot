// Package migrations embeds the goose SQL migrations of the werbot schema.
package migrations

import "embed"

// FS holds every *.sql migration, applied by db.Migrate.
//
//go:embed *.sql
var FS embed.FS
