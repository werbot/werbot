package identity

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/identity/dbq"
)

// ceremony binds a webauthn_ceremony row: kind is register, login, stepup or
// challenge; ref names what the ceremony confirms (empty for register).
type ceremony struct {
	orgID, userID uuid.UUID
	kind, ref     string
}

// saveCeremony removes the user's spent and expired ceremonies, stores
// session for ttl and returns options as JSON for the browser.
func saveCeremony(ctx context.Context, st Store, c ceremony, ttl time.Duration, options any, session *webauthn.SessionData) ([]byte, error) {
	if err := st.DeleteStaleCeremonies(ctx, c.userID); err != nil {
		return nil, err
	}
	data, err := json.Marshal(session)
	if err != nil {
		return nil, err
	}
	if err := st.CreateCeremony(ctx, dbq.CreateCeremonyParams{OrgID: c.orgID, UserID: c.userID, Kind: c.kind, Ref: c.ref,
		SessionData: data, TtlSecs: ttl.Seconds()}); err != nil {
		return nil, err
	}
	return json.Marshal(options)
}

// useCeremony spends the newest open ceremony c in the effect transaction;
// none (spent, expired, another user, kind or ref) is ErrInvalidCode.
func useCeremony(ctx context.Context, st Store, c ceremony) (webauthn.SessionData, error) {
	var session webauthn.SessionData
	data, err := st.UseCeremony(ctx, dbq.UseCeremonyParams{OrgID: c.orgID, UserID: c.userID, Kind: c.kind, Ref: c.ref})
	if errors.Is(err, pgx.ErrNoRows) {
		return session, ErrInvalidCode
	}
	if err != nil {
		return session, err
	}
	return session, json.Unmarshal(data, &session)
}
