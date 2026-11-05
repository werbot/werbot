package factory

import (
	"encoding/json"
	"time"

	"github.com/stretchr/testify/require"
)

// TokenParams is the set of optional and required values for a token.
type TokenParams struct {
	ID                              string
	Section, Action, Status         int16
	ProfileID, ProjectID, SchemeID  string
	Data                            map[string]any
	ExpiredAt, UpdatedAt, CreatedAt *time.Time
}

// Token is a created token row.
type Token struct {
	ID                      string
	Section, Action, Status int16
}

// Token inserts one token row and returns it.
func (b *Builder) Token(t TokenParams) Token {
	b.t.Helper()

	t.ID = uuidOrNew(t.ID)
	if t.Data == nil {
		t.Data = map[string]any{}
	}

	data, err := json.Marshal(t.Data)
	require.NoError(b.t, err, "token: marshal data")

	_, err = b.db.Exec(`
		INSERT INTO "token" (
			"id", "section", "action", "status",
			"profile_id", "project_id", "scheme_id",
			"data", "expired_at", "updated_at", "created_at"
		) VALUES (
			$1, $2, $3, $4,
			NULLIF($5, '')::uuid, NULLIF($6, '')::uuid, NULLIF($7, '')::uuid,
			$8, $9, $10, COALESCE($11, NOW())
		)
	`, t.ID, t.Section, t.Action, t.Status,
		t.ProfileID, t.ProjectID, t.SchemeID,
		data, t.ExpiredAt, t.UpdatedAt, t.CreatedAt)
	require.NoError(b.t, err, "token: insert")

	return Token{
		ID:      t.ID,
		Section: t.Section,
		Action:  t.Action,
		Status:  t.Status,
	}
}
