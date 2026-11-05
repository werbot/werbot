package factory

import (
	"encoding/json"
	"time"

	"github.com/stretchr/testify/require"
)

// EventParams is the set of optional and required values for an event.
type EventParams struct {
	ID, RelatedID               string
	PrimeSection, Section, Type int16
	Session                     map[string]any
	SessionID, UserAgent, IP    string
	Data                        map[string]any
	CreatedAt                   *time.Time
}

// Event is a created event row.
type Event struct {
	ID, RelatedID               string
	PrimeSection, Section, Type int16
}

// Event inserts one event row and returns it.
func (b *Builder) Event(e EventParams) Event {
	b.t.Helper()

	e.ID = uuidOrNew(e.ID)
	if e.Session == nil {
		e.Session = map[string]any{
			"id":         uuidOrNew(e.SessionID),
			"user_agent": stringOr(e.UserAgent, "test-agent"),
			"ip":         stringOr(e.IP, "192.0.2.1"),
		}
	}
	if e.Data == nil {
		e.Data = map[string]any{}
	}

	session, err := json.Marshal(e.Session)
	require.NoError(b.t, err, "event: marshal session")
	data, err := json.Marshal(e.Data)
	require.NoError(b.t, err, "event: marshal data")

	_, err = b.db.Exec(`
		INSERT INTO "event" (
			"id", "related_id", "prime_section", "section", "type",
			"session", "data", "created_at"
		) VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE($8, NOW()))
	`, e.ID, e.RelatedID, e.PrimeSection, e.Section, e.Type, session, data, e.CreatedAt)
	require.NoError(b.t, err, "event: insert")

	return Event{
		ID:           e.ID,
		RelatedID:    e.RelatedID,
		PrimeSection: e.PrimeSection,
		Section:      e.Section,
		Type:         e.Type,
	}
}

// SessionParams is the set of optional and required values for a session.
type SessionParams struct {
	ID, MemberID string
	Status       *bool
	Message      string
	CreatedAt    *time.Time
}

// Session is a created session row.
type Session struct{ ID, MemberID string }

// Session inserts one session row and returns it.
func (b *Builder) Session(s SessionParams) Session {
	b.t.Helper()

	s.ID = uuidOrNew(s.ID)
	status := boolOr(s.Status, false)

	_, err := b.db.Exec(`
		INSERT INTO "session" ("id", "member_id", "status", "message", "created_at")
		VALUES ($1, $2, $3, $4, COALESCE($5, NOW()))
	`, s.ID, s.MemberID, status, s.Message, s.CreatedAt)
	require.NoError(b.t, err, "session: insert")

	return Session{ID: s.ID, MemberID: s.MemberID}
}
