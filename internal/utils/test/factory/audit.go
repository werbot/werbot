package factory

import (
	"time"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/pkg/uuid"
)

// AuditParams is the set of optional and required values for an audit.
type AuditParams struct {
	ID, MemberID                      string
	TimeStart, TimeEnd                *time.Time
	Version, Width, Height            int16
	Duration                          float64
	Command, Title, EnvTerm, EnvShell string
	Session, ClientIP                 string
}

// Audit is a created audit row.
type Audit struct{ ID, MemberID string }

// Audit inserts one audit row and returns it.
func (b *Builder) Audit(a AuditParams) Audit {
	b.t.Helper()

	a.ID = uuidOrNew(a.ID)
	if a.Version == 0 {
		a.Version = 1
	}
	if a.Width == 0 {
		a.Width = 80
	}
	if a.Height == 0 {
		a.Height = 24
	}
	a.Command = stringOr(a.Command, "command")
	a.Title = stringOr(a.Title, "audit")
	a.EnvTerm = stringOr(a.EnvTerm, "xterm-256color")
	a.EnvShell = stringOr(a.EnvShell, "/bin/bash")
	a.Session = stringOr(a.Session, uuid.New())
	a.ClientIP = stringOr(a.ClientIP, "127.0.0.1")

	_, err := b.db.Exec(`
		INSERT INTO "audit" (
			"id", "member_id", "time_start", "time_end",
			"version", "width", "height", "duration",
			"command", "title", "env_term", "env_shell", "session", "client_ip"
		) VALUES ($1, $2, COALESCE($3, NOW()), $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, a.ID, a.MemberID, a.TimeStart, a.TimeEnd,
		a.Version, a.Width, a.Height, a.Duration,
		a.Command, a.Title, a.EnvTerm, a.EnvShell, a.Session, a.ClientIP)
	require.NoError(b.t, err, "audit: insert")

	return Audit{ID: a.ID, MemberID: a.MemberID}
}

// AuditRecordParams is the set of optional and required values for an audit record.
type AuditRecordParams struct {
	ID, AuditID  string
	Duration     float64
	Screen, Type string
}

// AuditRecord is a created audit record row.
type AuditRecord struct{ ID, AuditID string }

// AuditRecord inserts one audit record row and returns it.
func (b *Builder) AuditRecord(r AuditRecordParams) AuditRecord {
	b.t.Helper()

	r.ID = uuidOrNew(r.ID)

	_, err := b.db.Exec(`
		INSERT INTO "audit_record" ("id", "audit_id", "duration", "screen", "type")
		VALUES ($1, $2, $3, $4, $5)
	`, r.ID, r.AuditID, r.Duration, r.Screen, r.Type)
	require.NoError(b.t, err, "audit record: insert")

	return AuditRecord{ID: r.ID, AuditID: r.AuditID}
}
