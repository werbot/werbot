package identity

import (
	"context"
	"net/netip"
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
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

// testBinding is a key binding as the gateway sends it.
var testBinding = "key:SHA256:" + strings.Repeat("A", 43)

// challengeEnv is a seeded organization whose member p gets the challenges
// of instance f.InstanceID on f.ResourceID.
type challengeEnv struct {
	svc  *Service
	pool *pgxpool.Pool
	f    dbtest.Fixture
	p    principal.Principal
}

func newChallengeEnv(t *testing.T) challengeEnv {
	t.Helper()
	svc, pool, _ := newTestService(t)
	f := dbtest.Seed(t, pool)
	return challengeEnv{svc: svc, pool: pool, f: f, p: f.Principal(f.MemberID)}
}

func (e challengeEnv) input(strength string) MFAChallengeInput {
	return MFAChallengeInput{OrgID: e.f.OrgID, UserID: e.p.UserID, InstanceID: &e.f.InstanceID, ResourceID: &e.f.ResourceID,
		SSHSessionID: []byte("ssh-session"), ClientIP: netip.MustParseAddr("198.51.100.4"), Binding: testBinding, Strength: strength}
}

func (e challengeEnv) create(t *testing.T, strength string) MFAChallenge {
	t.Helper()
	c, err := e.svc.CreateMFAChallenge(context.Background(), e.input(strength))
	require.NoError(t, err)
	return c
}

func (e challengeEnv) status(t *testing.T, id uuid.UUID) string {
	t.Helper()
	var s string
	require.NoError(t, e.pool.QueryRow(context.Background(), "SELECT status FROM mfa_challenge WHERE id = $1", id).Scan(&s))
	return s
}

func (e challengeEnv) expire(t *testing.T, id uuid.UUID) {
	t.Helper()
	_, err := e.pool.Exec(context.Background(), "UPDATE mfa_challenge SET expires_at = now() - interval '1 second' WHERE id = $1", id)
	require.NoError(t, err)
}

// listenFor returns the payload of the first NOTIFY on channel within d; the
// channel closes empty when none came.
func listenFor(t *testing.T, pool *pgxpool.Pool, channel string, d time.Duration) <-chan string {
	t.Helper()
	conn, err := pool.Acquire(context.Background())
	require.NoError(t, err)
	_, err = conn.Exec(context.Background(), "LISTEN "+pgx.Identifier{channel}.Sanitize())
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	out := make(chan string, 1)
	go func() {
		defer conn.Release()
		defer cancel()
		if n, err := conn.Conn().WaitForNotification(ctx); err == nil {
			out <- n.Payload
		}
		close(out)
	}()
	return out
}

func TestCreateMFAChallenge(t *testing.T) {
	e := newChallengeEnv(t)
	ctx := context.Background()
	c := e.create(t, "any")
	assert.Regexp(t, `^[BCDFGHJKLMNPQRSTVWXZ]{4}-[BCDFGHJKLMNPQRSTVWXZ]{4}$`, c.Code)
	assert.Equal(t, "https://werbot.test/mfa/"+c.ID.String(), c.URL)
	assert.Equal(t, "pending", c.Status)
	var ttl time.Duration
	var binding, ip string
	var instance uuid.UUID
	require.NoError(t, e.pool.QueryRow(ctx, `SELECT expires_at - created_at, binding, host(client_ip), gateway_instance_id
		FROM mfa_challenge WHERE id = $1 AND org_id = $2 AND user_id = $3`, c.ID, e.f.OrgID, e.p.UserID).Scan(&ttl, &binding, &ip, &instance))
	assert.Equal(t, ChallengeTTL, ttl)
	assert.Equal(t, []string{testBinding, "198.51.100.4"}, []string{binding, ip})
	assert.Equal(t, e.f.InstanceID, instance)
	assert.WithinDuration(t, time.Now().Add(ChallengeTTL), c.ExpiresAt, 5*time.Second)
	assert.NotEqual(t, c.Code, e.create(t, "webauthn").Code, "every challenge draws its own code")

	other := dbtest.Seed(t, e.pool)
	for name, edit := range map[string]func(in *MFAChallengeInput){
		"another organization's user":     func(in *MFAChallengeInput) { in.UserID = other.MemberID },
		"unknown user":                    func(in *MFAChallengeInput) { in.UserID = uuid.New() },
		"another organization's resource": func(in *MFAChallengeInput) { in.ResourceID = &other.ResourceID },
		"another organization's instance": func(in *MFAChallengeInput) { in.InstanceID = &other.InstanceID },
	} {
		in := e.input("any")
		edit(&in)
		_, err := e.svc.CreateMFAChallenge(ctx, in)
		require.ErrorIs(t, err, apperr.ErrNotFound, name)
	}
	for name, edit := range map[string]func(in *MFAChallengeInput){
		"strength": func(in *MFAChallengeInput) { in.Strength = "sms" },
		"binding":  func(in *MFAChallengeInput) { in.Binding = "" },
	} {
		in := e.input("any")
		edit(&in)
		_, err := e.svc.CreateMFAChallenge(ctx, in)
		require.ErrorIs(t, err, apperr.ErrInvalidArgument, name)
	}
}

// A challenge is readable only by its own user, only while it is pending and
// unexpired; the owner sees what it confirms and, with a passkey, the
// assertion options.
func TestGetMFAChallengeOwnOnly(t *testing.T) {
	e := newChallengeEnv(t)
	ctx := context.Background()
	c := e.create(t, "any")
	got, err := e.svc.GetMFAChallenge(ctx, e.p, c.ID)
	require.NoError(t, err)
	assert.Equal(t, MFAChallenge{ID: c.ID, Code: c.Code, Status: "pending", Strength: "any", Host: "test", Resource: "web",
		ClientIP: netip.MustParseAddr("198.51.100.4"), ExpiresAt: got.ExpiresAt, URL: c.URL}, got, "no passkey: no options")
	assert.WithinDuration(t, c.ExpiresAt, got.ExpiresAt, time.Millisecond)

	registerPasskey(t, e.svc, e.p, webauthntest.New(t, testOrigin))
	got, err = e.svc.GetMFAChallenge(ctx, e.p, c.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, got.WebAuthnOptions)
	assert.Equal(t, 1, countRows(t, e.pool, "SELECT count(*) FROM webauthn_ceremony WHERE kind = 'challenge' AND ref = $1", c.ID.String()))

	other := dbtest.Seed(t, e.pool)
	for name, p := range map[string]principal.Principal{
		"another member":       e.f.Principal(e.f.OtherMemberID),
		"the owner":            e.f.Principal(e.f.OwnerID),
		"another organization": {UserID: e.p.UserID, OrgID: other.OrgID, Role: principal.RoleMember},
	} {
		_, err := e.svc.GetMFAChallenge(ctx, p, c.ID)
		require.ErrorIs(t, err, apperr.ErrNotFound, name)
	}
	agent := e.p
	agent.AgentID, agent.RunID = &uuid.UUID{1}, &uuid.UUID{2}
	_, err = e.svc.GetMFAChallenge(ctx, agent, c.ID)
	require.ErrorIs(t, err, apperr.ErrPermissionDenied, "agents never see a challenge")

	_, err = e.pool.Exec(ctx, "UPDATE mfa_challenge SET status = 'confirmed' WHERE id = $1", c.ID)
	require.NoError(t, err)
	_, err = e.svc.GetMFAChallenge(ctx, e.p, c.ID)
	require.ErrorIs(t, err, apperr.ErrFailedPrecondition, "confirmed")
	expired := e.create(t, "any")
	e.expire(t, expired.ID)
	_, err = e.svc.GetMFAChallenge(ctx, e.p, expired.ID)
	require.ErrorIs(t, err, apperr.ErrFailedPrecondition, "expired")
}

func TestConfirmMFAChallenge(t *testing.T) {
	e := newChallengeEnv(t)
	ctx := context.Background()
	a := webauthntest.New(t, testOrigin)
	registerPasskey(t, e.svc, e.p, a)
	c := e.create(t, "webauthn")
	got, err := e.svc.GetMFAChallenge(ctx, e.p, c.ID)
	require.NoError(t, err)

	err = e.svc.ConfirmMFAChallenge(ctx, e.f.Principal(e.f.OtherMemberID), c.ID, a.Get(got.WebAuthnOptions))
	require.ErrorIs(t, err, apperr.ErrNotFound, "another member's challenge")
	agent := e.p
	agent.AgentID, agent.RunID = &uuid.UUID{1}, &uuid.UUID{2}
	require.ErrorIs(t, e.svc.ConfirmMFAChallenge(ctx, agent, c.ID, a.Get(got.WebAuthnOptions)), apperr.ErrPermissionDenied, "agents cannot confirm")
	err = e.svc.ConfirmMFAChallenge(ctx, e.p, c.ID, withOrigin(a, "https://evil.test", got.WebAuthnOptions))
	require.ErrorIs(t, err, ErrInvalidCode)
	assert.Equal(t, "pending", e.status(t, c.ID))
	assert.Equal(t, 1, countAudit(t, e.pool, "auth.mfa.fail", `{"method":"webauthn"}`))

	notify := listenFor(t, e.pool, "werbot_mfa_confirmed", 5*time.Second)
	require.NoError(t, e.svc.ConfirmMFAChallenge(ctx, e.p, c.ID, a.Get(got.WebAuthnOptions)), "a failed assertion spends nothing")
	assert.Equal(t, e.f.InstanceID.String()+":"+c.ID.String(), <-notify)
	assert.Equal(t, "confirmed", e.status(t, c.ID))
	assert.Equal(t, 1, countRows(t, e.pool, `SELECT count(*) FROM mfa_verification
		WHERE org_id = $1 AND user_id = $2 AND binding = $3 AND method = 'webauthn'`, e.f.OrgID, e.p.UserID, testBinding))
	assert.Equal(t, 1, countAudit(t, e.pool, "mfa.challenge.confirm", `{"challenge_id":"`+c.ID.String()+`","method":"webauthn"}`))

	err = e.svc.ConfirmMFAChallenge(ctx, e.p, c.ID, a.Get(got.WebAuthnOptions))
	require.ErrorIs(t, err, apperr.ErrFailedPrecondition, "a challenge is confirmed once")
}

// An expired challenge is never confirmed: no verification, no NOTIFY.
func TestConfirmExpiredChallenge(t *testing.T) {
	e := newChallengeEnv(t)
	ctx := context.Background()
	a := webauthntest.New(t, testOrigin)
	registerPasskey(t, e.svc, e.p, a)
	c := e.create(t, "any")
	got, err := e.svc.GetMFAChallenge(ctx, e.p, c.ID)
	require.NoError(t, err)
	e.expire(t, c.ID)

	notify := listenFor(t, e.pool, "werbot_mfa_confirmed", time.Second)
	err = e.svc.ConfirmMFAChallenge(ctx, e.p, c.ID, a.Get(got.WebAuthnOptions))
	require.ErrorIs(t, err, apperr.ErrFailedPrecondition)
	_, sent := <-notify
	assert.False(t, sent, "no NOTIFY for an expired challenge")
	assert.Zero(t, countRows(t, e.pool, "SELECT count(*) FROM mfa_verification"))
	assert.Zero(t, countAudit(t, e.pool, "mfa.challenge.confirm", `{}`))
}
