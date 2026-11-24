package migrations_test

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/migrations"
)

// insertIDP inserts an OIDC identity provider of org and returns its id.
func insertIDP(t *testing.T, pool *pgxpool.Pool, org uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(), `INSERT INTO idp_oidc (org_id, name, issuer, client_id, client_secret_ciphertext, key_id)
		VALUES ($1, $2, 'https://idp.example.com', 'werbot', '\x00', 'k') RETURNING id`, org, uuid.NewString()[:8]).Scan(&id))
	return id
}

// insertChallenge inserts a pending MFA challenge of user in org on instance
// (nil for a step-up) and resource (may be nil) and returns the insert error.
func insertChallenge(ctx context.Context, pool *pgxpool.Pool, org, user uuid.UUID, instance, resource *uuid.UUID) error {
	_, err := pool.Exec(ctx, `INSERT INTO mfa_challenge (org_id, user_id, code, binding, strength, expires_at, gateway_instance_id, resource_id)
		VALUES ($1, $2, 'BCDF-GHJK', 'key:SHA256:x', 'any', now() + interval '1 minute', $3, $4)`, org, user, instance, resource)
	return err
}

// TestMigrations14: the stage 14 CHECKs refuse values outside their lists and
// ranges, the defaults hold, an mfa_login token is accepted, and 00200–00205
// go down to 00182 (with such a token stored) and up again.
func TestMigrations14(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	f := dbtest.Seed(t, pool)

	rule := func(maxAge, strength string) error {
		_, err := pool.Exec(ctx, `INSERT INTO mfa_rule (org_id, name, subjects, resource_selector, max_age, strength)
			VALUES ($1, $2, '{}', '{}', $3::interval, $4)`, f.OrgID, uuid.NewString()[:8], maxAge, strength)
		return err
	}
	pgCode(t, rule("29 seconds", "any"), "23514")
	pgCode(t, rule("25 hours", "any"), "23514")
	pgCode(t, rule("1 minute", "bogus"), "23514")
	require.NoError(t, rule("30 seconds", "any"))
	require.NoError(t, rule("24 hours", "webauthn"))
	for _, q := range []string{
		`INSERT INTO mfa_rule (org_id, name, subjects, resource_selector, max_age) VALUES ($1, 'subjects', '[]', '{}', '1 minute')`,
		`INSERT INTO mfa_rule (org_id, name, subjects, resource_selector, max_age) VALUES ($1, 'selector', '{}', '[]', '1 minute')`,
	} {
		_, err := pool.Exec(ctx, q, f.OrgID)
		pgCode(t, err, "23514")
	}

	member := []any{f.OrgID, f.MemberID}
	for name, c := range map[string]struct {
		q    string
		args []any
	}{
		"require_mfa some":     {"UPDATE org_setting SET require_mfa = 'some' WHERE org_id = $1", []any{f.OrgID}},
		"challenge status":     {"INSERT INTO mfa_challenge (org_id, user_id, code, binding, strength, status, expires_at) VALUES ($1, $2, 'C', 'b', 'any', 'used', now())", member},
		"challenge strength":   {"INSERT INTO mfa_challenge (org_id, user_id, code, binding, strength, expires_at) VALUES ($1, $2, 'C', 'b', 'bogus', now())", member},
		"verification method":  {"INSERT INTO mfa_verification (org_id, user_id, binding, method) VALUES ($1, $2, 'b', 'sms')", member},
		"ceremony kind":        {"INSERT INTO webauthn_ceremony (org_id, user_id, kind, session_data, expires_at) VALUES ($1, $2, 'bogus', '{}', now())", member},
		"one-time token kind":  {"INSERT INTO one_time_token (token_hash, kind, org_id, user_id, expires_at) VALUES ('\\x01', 'bogus', $1, $2, now())", member},
		"webauthn name length": {"INSERT INTO webauthn_credential (user_id, name, credential_id, public_key, aaguid) VALUES ($1, '', '\\x01', '\\x01', '\\x00')", []any{f.MemberID}},
	} {
		_, err := pool.Exec(ctx, c.q, c.args...)
		require.Error(t, err, name)
		pgCode(t, err, "23514")
	}
	_, err := pool.Exec(ctx, `INSERT INTO one_time_token (token_hash, kind, org_id, user_id, expires_at)
		VALUES ('\x02', 'mfa_login', $1, $2, now() + interval '5 minutes')`, f.OrgID, f.MemberID)
	require.NoError(t, err)

	idp := insertIDP(t, pool, f.OrgID)
	var scopes []string
	var mapping string
	var signup, enabled bool
	require.NoError(t, pool.QueryRow(ctx, "SELECT scopes, claims_mapping::text, allow_signup, enabled FROM idp_oidc WHERE id = $1", idp).
		Scan(&scopes, &mapping, &signup, &enabled))
	assert.Equal(t, []string{"openid", "email", "profile"}, scopes, "groups is a scope only some IdPs know")
	assert.Equal(t, "[]", mapping)
	assert.False(t, signup)
	assert.True(t, enabled)

	var requireMFA, status string
	require.NoError(t, pool.QueryRow(ctx, "SELECT require_mfa FROM org_setting WHERE org_id = $1", f.OrgID).Scan(&requireMFA))
	assert.Equal(t, "off", requireMFA)
	require.NoError(t, insertChallenge(ctx, pool, f.OrgID, f.MemberID, &f.InstanceID, nil))
	require.NoError(t, pool.QueryRow(ctx, "SELECT status FROM mfa_challenge WHERE user_id = $1", f.MemberID).Scan(&status))
	assert.Equal(t, "pending", status)
	var lastStep int64
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO totp_secret (user_id, ciphertext, key_id) VALUES ($1, '\x00', 'k') RETURNING last_step`,
		f.MemberID).Scan(&lastStep))
	assert.Zero(t, lastStep)

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	require.NoError(t, err)
	tables := []string{"idp_oidc", "user_idp_link", "webauthn_credential", "totp_secret", "mfa_recovery_code",
		"mfa_verification", "mfa_challenge", "mfa_rule", "rate_limit", "webauthn_ceremony"}
	exist := func() (n int) {
		for _, tbl := range tables {
			var ok bool
			require.NoError(t, pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", tbl).Scan(&ok))
			if ok {
				n++
			}
		}
		return n
	}
	_, err = p.DownTo(ctx, 182)
	require.NoError(t, err)
	assert.Zero(t, exist())
	for _, q := range []string{"SELECT mfa_at FROM user_session", "SELECT require_mfa FROM org_setting", "SELECT user_session_id FROM stream_ticket"} {
		_, err = pool.Exec(ctx, q)
		pgCode(t, err, "42703")
	}
	_, err = p.Up(ctx)
	require.NoError(t, err)
	assert.Equal(t, len(tables), exist())
}

// TestStage14CrossOrgRejected: org-level stage 14 rows reference only their
// own organization (another org's IdP, gateway instance or resource, or a
// non-member user, is 23503); they go with the membership, instance and user
// session they name, while user-level factors stay until the user goes.
func TestStage14CrossOrgRejected(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	a, b := dbtest.Seed(t, pool), dbtest.Seed(t, pool)
	idpA, idpB := insertIDP(t, pool, a.OrgID), insertIDP(t, pool, b.OrgID)

	link := func(idp, org, user uuid.UUID) error {
		_, err := pool.Exec(ctx, "INSERT INTO user_idp_link (user_id, idp_id, org_id, subject) VALUES ($1, $2, $3, $4)",
			user, idp, org, uuid.NewString())
		return err
	}
	pgCode(t, link(idpB, a.OrgID, a.MemberID), "23503")
	pgCode(t, link(idpB, b.OrgID, a.MemberID), "23503")
	require.NoError(t, link(idpA, a.OrgID, a.MemberID))

	pgCode(t, insertChallenge(ctx, pool, a.OrgID, a.MemberID, &b.InstanceID, nil), "23503")
	pgCode(t, insertChallenge(ctx, pool, a.OrgID, a.MemberID, &a.InstanceID, &b.ResourceID), "23503")
	pgCode(t, insertChallenge(ctx, pool, a.OrgID, b.MemberID, &a.InstanceID, nil), "23503")
	require.NoError(t, insertChallenge(ctx, pool, a.OrgID, a.MemberID, &a.InstanceID, &a.ResourceID))
	require.NoError(t, insertChallenge(ctx, pool, a.OrgID, a.MemberID, nil, nil))
	require.NoError(t, insertChallenge(ctx, pool, a.OrgID, a.OtherMemberID, &a.OtherInstanceID, nil))
	for _, q := range []string{
		"INSERT INTO mfa_verification (org_id, user_id, binding, method) VALUES ($1, $2, 'b', 'totp')",
		"INSERT INTO webauthn_ceremony (org_id, user_id, kind, session_data, expires_at) VALUES ($1, $2, 'login', '{}', now())",
	} {
		_, err := pool.Exec(ctx, q, a.OrgID, a.MemberID)
		require.NoError(t, err, q)
	}
	_, err := pool.Exec(ctx, `INSERT INTO totp_secret (user_id, ciphertext, key_id) VALUES ($1, '\x00', 'k')`, a.MemberID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "INSERT INTO mfa_verification (org_id, user_id, binding, method) VALUES ($1, $2, 'b', 'totp')", a.OrgID, b.MemberID)
	pgCode(t, err, "23503")

	count := func(q string, args ...any) (n int) {
		require.NoError(t, pool.QueryRow(ctx, q, args...).Scan(&n))
		return n
	}
	_, err = pool.Exec(ctx, "DELETE FROM resource WHERE id = $1", a.ResourceID)
	require.NoError(t, err)
	assert.Equal(t, 2, count("SELECT count(*) FROM mfa_challenge WHERE user_id = $1 AND resource_id IS NULL", a.MemberID),
		"a deleted resource is cleared from the challenge, the challenge stays")
	_, err = pool.Exec(ctx, "DELETE FROM gateway_instance WHERE id = $1", a.OtherInstanceID)
	require.NoError(t, err)
	assert.Zero(t, count("SELECT count(*) FROM mfa_challenge WHERE user_id = $1", a.OtherMemberID), "challenges go with their instance")

	_, err = pool.Exec(ctx, "DELETE FROM membership WHERE org_id = $1 AND user_id = $2", a.OrgID, a.MemberID)
	require.NoError(t, err)
	for _, tbl := range []string{"user_idp_link", "mfa_verification", "mfa_challenge", "webauthn_ceremony"} {
		assert.Zero(t, count("SELECT count(*) FROM "+tbl+" WHERE user_id = $1", a.MemberID), tbl)
	}
	assert.Equal(t, 1, count("SELECT count(*) FROM totp_secret WHERE user_id = $1", a.MemberID), "factors are user-level")
	_, err = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", a.MemberID)
	require.NoError(t, err)
	assert.Zero(t, count("SELECT count(*) FROM totp_secret WHERE user_id = $1", a.MemberID))

	var us uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO user_session (token_hash, user_id, org_id, expires_at)
		VALUES ('\x01', $1, $2, now() + interval '1 hour') RETURNING id`, a.OwnerID, a.OrgID).Scan(&us))
	res := dbtest.Resource(t, pool, a.OrgID, "web-2", nil)
	require.NoError(t, insertTicket(ctx, pool, a.OrgID, "web", nil, &res, a.OwnerID, a.InstanceID))
	_, err = pool.Exec(ctx, "UPDATE stream_ticket SET user_session_id = $1 WHERE org_id = $2", us, a.OrgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "DELETE FROM user_session WHERE id = $1", us)
	require.NoError(t, err)
	assert.Zero(t, count("SELECT count(*) FROM stream_ticket WHERE org_id = $1", a.OrgID), "a ticket goes with its user session")
}

// TestMFARuleNotifiesOrgChanged: an mfa_rule change notifies werbot_org_changed
// with its organization; the other stage 14 tables do not notify.
func TestMFARuleNotifiesOrgChanged(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	f := dbtest.Seed(t, pool)
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, "LISTEN werbot_org_changed")
	require.NoError(t, err)

	var rule uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO mfa_rule (org_id, name, subjects, resource_selector, max_age)
		VALUES ($1, 'r', '{}', '{}', '1 minute') RETURNING id`, f.OrgID).Scan(&rule)) // notifies
	for _, q := range []string{
		"UPDATE mfa_rule SET strength = 'webauthn' WHERE id = $1", // notifies
		"DELETE FROM mfa_rule WHERE id = $1",                      // notifies
	} {
		_, err = pool.Exec(ctx, q, rule)
		require.NoError(t, err)
	}
	insertIDP(t, pool, f.OrgID)                                                             // no
	require.NoError(t, insertChallenge(ctx, pool, f.OrgID, f.MemberID, &f.InstanceID, nil)) // no
	for _, q := range []string{
		"INSERT INTO mfa_verification (org_id, user_id, binding, method) VALUES ($1, $2, 'b', 'totp')",
		"INSERT INTO webauthn_ceremony (org_id, user_id, kind, session_data, expires_at) VALUES ($1, $2, 'login', '{}', now())",
		"INSERT INTO rate_limit (key, window_start, count) VALUES ($1::uuid::text || $2::uuid::text, now(), 1)",
	} {
		_, err = pool.Exec(ctx, q, f.OrgID, f.MemberID) // no
		require.NoError(t, err, q)
	}

	other := dbtest.Org(t, pool) // marks the end: notifications arrive in commit order
	got := 0
	for {
		wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		n, err := conn.Conn().WaitForNotification(wctx)
		cancel()
		require.NoError(t, err)
		if n.Payload == other.String() {
			break
		}
		assert.Equal(t, f.OrgID.String(), n.Payload)
		got++
	}
	assert.Equal(t, 3, got)
}

// TestStage14SecretColumnsSealed: secrets of the stage 14 tables are stored
// only as ciphertext or hashes (bytea); no column holds them in clear text.
func TestStage14SecretColumnsSealed(t *testing.T) {
	pool, _ := dbtest.New(t)
	rows, err := pool.Query(context.Background(), `SELECT table_name || '.' || column_name, data_type FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name IN ('idp_oidc', 'user_idp_link', 'totp_secret', 'mfa_recovery_code')`)
	require.NoError(t, err)
	cols := map[string]string{}
	for rows.Next() {
		var name, typ string
		require.NoError(t, rows.Scan(&name, &typ))
		cols[name] = typ
	}
	require.NoError(t, rows.Err())

	sealed := map[string]bool{
		"idp_oidc.client_secret_ciphertext": true, "user_idp_link.refresh_ciphertext": true,
		"totp_secret.ciphertext": true, "mfa_recovery_code.code_hash": true,
	}
	for name := range sealed {
		assert.Equal(t, "bytea", cols[name], name)
	}
	secretName := regexp.MustCompile(`secret|refresh|token|code|ciphertext`)
	for name := range cols {
		_, col, _ := strings.Cut(name, ".")
		if secretName.MatchString(col) {
			assert.True(t, sealed[name], "%s may hold a secret in clear text", name)
		}
	}
}
