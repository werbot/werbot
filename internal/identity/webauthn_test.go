package identity

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/identity/webauthntest"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// testOrigin is WebURL of newTestService: the RP ID is its host.
const testOrigin = "https://werbot.test"

// registerPasskey registers a's credential as p's passkey "YubiKey".
func registerPasskey(t *testing.T, svc *Service, p principal.Principal, a *webauthntest.Authenticator) Factor {
	t.Helper()
	opts, err := svc.BeginWebAuthnRegistration(context.Background(), p)
	require.NoError(t, err)
	f, err := svc.FinishWebAuthnRegistration(context.Background(), p, "YubiKey", a.Create(opts))
	require.NoError(t, err)
	return f
}

// beginLogin starts a login ceremony of p for kind and ref and returns its options.
func beginLogin(t *testing.T, svc *Service, p principal.Principal, kind, ref string) []byte {
	t.Helper()
	opts, err := svc.BeginWebAuthnLogin(context.Background(), p.OrgID, p.UserID, kind, ref, CeremonyTTL)
	require.NoError(t, err)
	return opts
}

// finishLogin runs FinishWebAuthnLogin in its own transaction, as the callers do.
func finishLogin(svc *Service, p principal.Principal, kind, ref string, assertion []byte) (uuid.UUID, error) {
	var id uuid.UUID
	err := dbx.InOrg(context.Background(), svc.pool, p.OrgID, p.UserID, func(tx pgx.Tx) (err error) {
		id, err = svc.FinishWebAuthnLogin(context.Background(), tx, p.OrgID, p.UserID, kind, ref, assertion)
		return err
	})
	return id, err
}

func countRows(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(), query, args...).Scan(&n))
	return n
}

func TestWebAuthnRegistration(t *testing.T) {
	svc, pool, p := newMFATest(t)
	ctx := context.Background()
	_, err := svc.BeginWebAuthnLogin(ctx, p.OrgID, p.UserID, "login", "", CeremonyTTL)
	require.ErrorIs(t, err, apperr.ErrFailedPrecondition, "no passkey yet")
	opts, err := svc.BeginWebAuthnRegistration(ctx, p)
	require.NoError(t, err)
	var o struct {
		PublicKey struct {
			RP   struct{ ID, Name string }
			User struct{ ID, Name string }
		}
	}
	require.NoError(t, json.Unmarshal(opts, &o))
	assert.Equal(t, "werbot.test", o.PublicKey.RP.ID)
	assert.Equal(t, "Werbot", o.PublicKey.RP.Name)
	assert.Equal(t, base64.RawURLEncoding.EncodeToString(p.UserID[:]), o.PublicKey.User.ID)
	assert.Equal(t, "alice@example.com", o.PublicKey.User.Name)
	assert.Equal(t, 1, countRows(t, pool, `SELECT count(*) FROM webauthn_ceremony WHERE user_id = $1 AND kind = 'register'
		AND expires_at - created_at = interval '5 minutes'`, p.UserID), "a ceremony lives 5 minutes")

	_, err = svc.FinishWebAuthnRegistration(ctx, p, "  ", []byte("{}"))
	require.ErrorIs(t, err, apperr.ErrInvalidArgument)

	a := webauthntest.New(t, testOrigin)
	f, err := svc.FinishWebAuthnRegistration(ctx, p, " YubiKey ", a.Create(opts))
	require.NoError(t, err)
	assert.Equal(t, factorWebAuthn, f.Kind)
	assert.Equal(t, "YubiKey", f.Name)
	id, err := uuid.Parse(f.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, countAudit(t, pool, "mfa.factor.add", `{"kind":"webauthn"}`))
	fs, err := svc.ListFactors(ctx, p)
	require.NoError(t, err)
	require.Len(t, fs, 1)
	assert.Equal(t, id.String(), fs[0].ID)

	opts, err = svc.BeginWebAuthnRegistration(ctx, p)
	require.NoError(t, err)
	assert.Contains(t, string(opts), `"excludeCredentials"`, "the user's passkeys are excluded")
	assert.Equal(t, 1, countRows(t, pool, "SELECT count(*) FROM webauthn_ceremony WHERE user_id = $1", p.UserID),
		"Begin removes the spent ceremony")
	_, err = svc.FinishWebAuthnRegistration(ctx, p, "Again", a.Create(opts))
	require.ErrorIs(t, err, apperr.ErrAlreadyExists, "the same credential twice")
}

func TestWebAuthnRegistrationWrongOrigin(t *testing.T) {
	svc, pool, p := newMFATest(t)
	ctx := context.Background()
	other := webauthntest.New(t, "https://other.test")
	wrongRP := webauthntest.New(t, testOrigin)
	wrongRP.RPID = "other.test"
	for name, a := range map[string]*webauthntest.Authenticator{"origin": other, "rp id": wrongRP} {
		opts, err := svc.BeginWebAuthnRegistration(ctx, p)
		require.NoError(t, err)
		_, err = svc.FinishWebAuthnRegistration(ctx, p, "YubiKey", a.Create(opts))
		require.ErrorIs(t, err, ErrInvalidCode, name)
	}
	_, err := svc.FinishWebAuthnRegistration(ctx, p, "YubiKey", []byte("not json"))
	require.ErrorIs(t, err, ErrInvalidCode)
	assert.Zero(t, countRows(t, pool, "SELECT count(*) FROM webauthn_credential"))
	assert.Zero(t, countAudit(t, pool, "mfa.factor.add", `{}`))
}

func TestWebAuthnCeremonySingleUse(t *testing.T) {
	svc, pool, p := newMFATest(t)
	ctx := context.Background()
	a := webauthntest.New(t, testOrigin)
	opts, err := svc.BeginWebAuthnRegistration(ctx, p)
	require.NoError(t, err)
	attestation := a.Create(opts)
	_, err = svc.FinishWebAuthnRegistration(ctx, p, "YubiKey", attestation)
	require.NoError(t, err)
	_, err = svc.FinishWebAuthnRegistration(ctx, p, "YubiKey", attestation)
	require.ErrorIs(t, err, ErrInvalidCode, "a spent ceremony")

	assertion := a.Get(beginLogin(t, svc, p, "login", "r1"))
	_, err = finishLogin(svc, p, "login", "r1", assertion)
	require.NoError(t, err)
	_, err = finishLogin(svc, p, "login", "r1", assertion)
	require.ErrorIs(t, err, ErrInvalidCode, "a spent ceremony")

	assertion = a.Get(beginLogin(t, svc, p, "login", "r2"))
	_, err = pool.Exec(ctx, "UPDATE webauthn_ceremony SET expires_at = now() - interval '1 second' WHERE ref = 'r2'")
	require.NoError(t, err)
	_, err = finishLogin(svc, p, "login", "r2", assertion)
	require.ErrorIs(t, err, ErrInvalidCode, "an expired ceremony")
	_, err = finishLogin(svc, p, "challenge", "never-begun", assertion)
	require.ErrorIs(t, err, ErrInvalidCode, "no ceremony")
}

func TestWebAuthnLoginAssertion(t *testing.T) {
	svc, pool, alice := newMFATest(t)
	bob := principal.Principal{UserID: seedUser(t, pool, alice.OrgID, "bob", principal.RoleMember), OrgID: alice.OrgID}
	a, b := webauthntest.New(t, testOrigin), webauthntest.New(t, testOrigin)
	f := registerPasskey(t, svc, alice, a)
	registerPasskey(t, svc, bob, b)

	id, err := finishLogin(svc, alice, "login", "r1", a.Get(beginLogin(t, svc, alice, "login", "r1")))
	require.NoError(t, err)
	assert.Equal(t, f.ID, id.String())
	var count int64
	var used *time.Time
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT sign_count, last_used_at FROM webauthn_credential WHERE id = $1", id).
		Scan(&count, &used))
	assert.Equal(t, int64(1), count)
	assert.NotNil(t, used)

	opts := beginLogin(t, svc, alice, "stepup", "r2")
	bobOpts := beginLogin(t, svc, bob, "stepup", "r2")
	for name, assertion := range map[string]func() []byte{
		"another ref":               func() []byte { return a.Get(beginLogin(t, svc, alice, "stepup", "r3")) },
		"another user's challenge":  func() []byte { return a.Get(bobOpts) },
		"another user's credential": func() []byte { return b.Get(opts) },
		"another origin":            func() []byte { return withOrigin(a, "https://other.test", opts) },
		"another rp id":             func() []byte { return withRPID(a, "other.test", opts) },
		"not an assertion":          func() []byte { return []byte("{}") },
	} {
		_, err := finishLogin(svc, alice, "stepup", "r2", assertion())
		require.ErrorIs(t, err, ErrInvalidCode, name)
	}
	_, err = finishLogin(svc, alice, "login", "r2", a.Get(opts))
	require.ErrorIs(t, err, ErrInvalidCode, "another kind")
}

func withOrigin(a *webauthntest.Authenticator, origin string, opts []byte) []byte {
	saved := a.Origin
	a.Origin = origin
	defer func() { a.Origin = saved }()
	return a.Get(opts)
}

func withRPID(a *webauthntest.Authenticator, rpID string, opts []byte) []byte {
	a.RPID = rpID
	defer func() { a.RPID = "" }()
	return a.Get(opts)
}

func TestWebAuthnLockedDevice(t *testing.T) {
	svc, pool, p := newMFATest(t)
	ctx := context.Background()
	a := webauthntest.New(t, testOrigin)
	f := registerPasskey(t, svc, p, a)
	other := seedOrg(t, pool, "Other")
	_, err := pool.Exec(ctx, "INSERT INTO lock (org_id, target_type, target_id, reason) VALUES ($1, 'mfa_device', $2, 'x')", other, f.ID)
	require.NoError(t, err)
	_, err = finishLogin(svc, p, "login", "", a.Get(beginLogin(t, svc, p, "login", "")))
	require.NoError(t, err, "a lock of another organization")

	_, err = pool.Exec(ctx, "INSERT INTO lock (org_id, target_type, target_id, reason) VALUES ($1, 'mfa_device', $2, 'x')", p.OrgID, f.ID)
	require.NoError(t, err)
	_, err = finishLogin(svc, p, "login", "", a.Get(beginLogin(t, svc, p, "login", "")))
	require.ErrorIs(t, err, apperr.ErrPermissionDenied)
	assert.ErrorContains(t, err, "mfa device is locked")

	_, err = pool.Exec(ctx, "UPDATE lock SET expires_at = now() - interval '1 second' WHERE org_id = $1", p.OrgID)
	require.NoError(t, err)
	_, err = finishLogin(svc, p, "login", "", a.Get(beginLogin(t, svc, p, "login", "")))
	require.NoError(t, err, "an expired lock")
}

// TestWebAuthnNotCountedTowardMFALimit: the mfa:<user_id> limit counts TOTP
// and recovery codes only (contract §3.6); a passkey works past it.
func TestWebAuthnNotCountedTowardMFALimit(t *testing.T) {
	svc, _, p := newMFATest(t)
	ctx := context.Background()
	a := webauthntest.New(t, testOrigin)
	registerPasskey(t, svc, p, a)
	for range MFAAttempts {
		require.NoError(t, svc.limit(ctx, "mfa:"+p.UserID.String(), MFAAttempts, MFAWindow))
	}
	require.ErrorIs(t, svc.limit(ctx, "mfa:"+p.UserID.String(), MFAAttempts, MFAWindow), apperr.ErrResourceExhausted)
	_, err := finishLogin(svc, p, "login", "", a.Get(beginLogin(t, svc, p, "login", "")))
	require.NoError(t, err)
}

func TestWebAuthnDisabledForIPHost(t *testing.T) {
	var logs bytes.Buffer
	svc, pool, _ := newTestService(t)
	svc = New(pool, nil, Config{WebURL: "http://127.0.0.1:1", Box: svc.cfg.Box,
		Logger: slog.New(slog.NewTextHandler(&logs, nil))})
	assert.Equal(t, 1, strings.Count(logs.String(), "level=WARN"), logs.String())
	assert.Contains(t, logs.String(), "passkeys")
	org := seedOrg(t, pool, "Acme")
	p := principal.Principal{UserID: seedUser(t, pool, org, "alice", principal.RoleMember), OrgID: org, Role: principal.RoleMember}
	ctx := context.Background()

	_, err := svc.BeginWebAuthnRegistration(ctx, p)
	require.ErrorIs(t, err, apperr.ErrFailedPrecondition)
	assert.ErrorContains(t, err, "passkeys need a domain name in WERBOT_WEB_URL")
	_, err = svc.FinishWebAuthnRegistration(ctx, p, "YubiKey", []byte("{}"))
	require.ErrorIs(t, err, apperr.ErrFailedPrecondition)
	_, err = svc.BeginWebAuthnLogin(ctx, p.OrgID, p.UserID, "login", "", CeremonyTTL)
	require.ErrorIs(t, err, apperr.ErrFailedPrecondition)
	_, err = finishLogin(svc, p, "login", "", []byte("{}"))
	require.ErrorIs(t, err, apperr.ErrFailedPrecondition)

	e, err := svc.EnrollTOTP(ctx, p)
	require.NoError(t, err, "TOTP stays available")
	require.NoError(t, svc.ConfirmTOTP(ctx, p, codeAt(t, e.Secret, currentStep())))

	assert.NotNil(t, newWebAuthn(Config{WebURL: "http://localhost:18080"}), "localhost is a domain name")
}
