package oauth_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/identity/oauth"
)

// newSession logs the member in over the loopback flow: the first tokens.
func newSession(t *testing.T, s stand) tokenResponse {
	t.Helper()
	code, verifier := login(t, s, s.fx.MemberID)
	tr := exchangeCode(t, s.srv, code, verifier, testRedirect)
	require.Equal(t, http.StatusOK, tr.Status, tr.Error)
	return tr
}

func refresh(t *testing.T, s stand, token string) tokenResponse {
	t.Helper()
	res, err := http.PostForm(s.srv.URL+"/oauth/token", url.Values{
		"grant_type": {"refresh_token"}, "client_id": {oauth.ClientCLI}, "refresh_token": {token},
	})
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	tr := tokenResponse{Status: res.StatusCode}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&tr))
	return tr
}

func revoke(t *testing.T, s stand, token string) int {
	t.Helper()
	res, err := http.PostForm(s.srv.URL+"/oauth/revoke", url.Values{"client_id": {oauth.ClientCLI}, "token": {token}})
	require.NoError(t, err)
	_ = res.Body.Close()
	return res.StatusCode
}

func exec(t *testing.T, s stand, sql string, args ...any) {
	t.Helper()
	_, err := s.pool.Exec(context.Background(), sql, args...)
	require.NoError(t, err)
}

// session reads the only cli_session: revoke reason ("" while live) and
// expires_at - now().
func session(t *testing.T, s stand) (reason string, ttl time.Duration) {
	t.Helper()
	require.NoError(t, s.pool.QueryRow(context.Background(),
		"SELECT coalesce(revoke_reason, ''), expires_at - now() FROM cli_session").Scan(&reason, &ttl))
	return reason, ttl
}

func requireInvalidGrant(t *testing.T, tr tokenResponse) {
	t.Helper()
	require.Equal(t, http.StatusBadRequest, tr.Status)
	require.Equal(t, "invalid_grant", tr.Error)
	require.Empty(t, tr.AccessToken)
	require.Empty(t, tr.RefreshToken)
}

// TestRefreshRotates: a refresh returns a new refresh token, an access token
// of the same cli_session (jti) and keeps the old hash to detect a replay.
func TestRefreshRotates(t *testing.T) {
	s := newStand(t)
	ctx := context.Background()
	r1 := newSession(t, s)
	c1, err := s.p.VerifyAccessToken(ctx, r1.AccessToken)
	require.NoError(t, err)
	exec(t, s, "UPDATE cli_session SET last_used_at = now() - interval '1 hour'")

	r2 := refresh(t, s, r1.RefreshToken)
	require.Equal(t, http.StatusOK, r2.Status, r2.Error)
	assert.Regexp(t, `^wrt_[A-Za-z0-9_-]{43}$`, r2.RefreshToken)
	assert.NotEqual(t, r1.RefreshToken, r2.RefreshToken)
	assert.InDelta(t, 600, r2.ExpiresIn, 2)
	c2, err := s.p.VerifyAccessToken(ctx, r2.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, c1.CLISessionID, c2.CLISessionID)
	assert.Equal(t, s.fx.OrgID, c2.OrgID)
	assert.Equal(t, s.fx.MemberID, c2.UserID)

	var (
		prev, cur []byte
		idle      time.Duration
	)
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT prev_refresh_hash, refresh_hash, now() - last_used_at FROM cli_session").
		Scan(&prev, &cur, &idle))
	h1, h2 := sha256.Sum256([]byte(r1.RefreshToken)), sha256.Sum256([]byte(r2.RefreshToken))
	assert.Equal(t, h1[:], prev)
	assert.Equal(t, h2[:], cur)
	assert.Less(t, idle, time.Minute)
}

// TestRefreshReuseRevokesFamily: presenting the previous refresh token again
// (a stolen copy, or a retry after a lost response) revokes the cli_session,
// audited as cli_session.revoke by the system; neither token works after.
func TestRefreshReuseRevokesFamily(t *testing.T) {
	s := newStand(t)
	ctx := context.Background()
	r1 := newSession(t, s)
	r2 := refresh(t, s, r1.RefreshToken)
	require.Equal(t, http.StatusOK, r2.Status, r2.Error)

	requireInvalidGrant(t, refresh(t, s, r1.RefreshToken))
	var revoked bool
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT revoked_at IS NOT NULL FROM cli_session").Scan(&revoked))
	assert.True(t, revoked)
	reason, _ := session(t, s)
	assert.Equal(t, "refresh_reuse", reason)
	requireInvalidGrant(t, refresh(t, s, r2.RefreshToken))

	var (
		kind, target, row string
		actor             *string
		data              map[string]any
	)
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT actor_kind, actor_user_id::text, target_id, data, e::text
		FROM audit_event e JOIN cli_session c ON c.id::text = e.target_id
		WHERE action = 'cli_session.revoke' AND e.org_id = $1`, s.fx.OrgID).Scan(&kind, &actor, &target, &data, &row))
	assert.Equal(t, "system", kind)
	assert.Nil(t, actor)
	assert.Equal(t, map[string]any{"reason": "refresh_reuse"}, data)
	assert.NotContains(t, row, r1.RefreshToken)
	assert.NotContains(t, row, r2.RefreshToken)
}

// TestRefreshParallelLoser: two refreshes of one token that both passed the
// lookup rotate once; the loser gets invalid_grant and the session stays live.
func TestRefreshParallelLoser(t *testing.T) {
	s := newStand(t)
	ctx := context.Background()
	r1 := newSession(t, s)

	// Hold the row so both rotations queue behind the lookup.
	lock, err := s.pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = lock.Rollback(ctx) }()
	_, err = lock.Exec(ctx, "SELECT 1 FROM cli_session FOR UPDATE")
	require.NoError(t, err)

	results := make([]tokenResponse, 2)
	var wg sync.WaitGroup
	for i := range results {
		wg.Go(func() { results[i] = refresh(t, s, r1.RefreshToken) })
	}
	require.Eventually(t, func() bool {
		var n int
		require.NoError(t, s.pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&n))
		return n == 2
	}, 10*time.Second, 10*time.Millisecond)
	require.NoError(t, lock.Commit(ctx))
	wg.Wait()

	ok := 0
	for _, r := range results {
		if r.Status == http.StatusOK {
			ok++
			continue
		}
		requireInvalidGrant(t, r)
	}
	assert.Equal(t, 1, ok)
	reason, _ := session(t, s)
	assert.Empty(t, reason, "the loser does not revoke")
}

// TestRefreshSlidingTTL: a refresh moves expires_at to now + cli_session_ttl;
// an idle-expired session is refused (and not revoked: no replay).
func TestRefreshSlidingTTL(t *testing.T) {
	s := newStand(t)
	r1 := newSession(t, s)
	exec(t, s, "UPDATE org_setting SET cli_session_ttl = '2 hours' WHERE org_id = $1", s.fx.OrgID)
	exec(t, s, "UPDATE cli_session SET expires_at = now() + interval '1 minute'")

	r2 := refresh(t, s, r1.RefreshToken)
	require.Equal(t, http.StatusOK, r2.Status, r2.Error)
	_, ttl := session(t, s)
	assert.InDelta(t, (2 * time.Hour).Seconds(), ttl.Seconds(), 60)

	exec(t, s, "UPDATE cli_session SET expires_at = now() - interval '1 second'")
	requireInvalidGrant(t, refresh(t, s, r2.RefreshToken))
	reason, _ := session(t, s)
	assert.Empty(t, reason)
}

// TestRefreshAbsoluteMax: expires_at never passes absolute_expires_at; past
// it the session is refused.
func TestRefreshAbsoluteMax(t *testing.T) {
	s := newStand(t)
	r1 := newSession(t, s)
	exec(t, s, "UPDATE cli_session SET absolute_expires_at = now() + interval '30 minutes'")

	r2 := refresh(t, s, r1.RefreshToken)
	require.Equal(t, http.StatusOK, r2.Status, r2.Error)
	var capped bool
	require.NoError(t, s.pool.QueryRow(context.Background(),
		"SELECT expires_at = absolute_expires_at FROM cli_session").Scan(&capped))
	assert.True(t, capped)

	exec(t, s, "UPDATE cli_session SET absolute_expires_at = now() - interval '1 second'")
	requireInvalidGrant(t, refresh(t, s, r2.RefreshToken))
}

// TestRefreshAfterMaxLowered: created_at + cli_session_max (current
// settings) caps a refresh, and a lowered cli_session_max ends older sessions
// at their next refresh.
func TestRefreshAfterMaxLowered(t *testing.T) {
	s := newStand(t)
	r1 := newSession(t, s)
	exec(t, s, "UPDATE cli_session SET created_at = now() - interval '3 days'")
	exec(t, s, "UPDATE org_setting SET cli_session_max = '4 days' WHERE org_id = $1", s.fx.OrgID)

	r2 := refresh(t, s, r1.RefreshToken)
	require.Equal(t, http.StatusOK, r2.Status, r2.Error)
	_, ttl := session(t, s)
	assert.InDelta(t, (24 * time.Hour).Seconds(), ttl.Seconds(), 60)

	exec(t, s, "UPDATE org_setting SET cli_session_max = '1 day' WHERE org_id = $1", s.fx.OrgID)
	requireInvalidGrant(t, refresh(t, s, r2.RefreshToken))
}

// TestRefreshAfterMemberRemoved: removing the member deletes its cli_session
// (cascade); its refresh token is refused.
func TestRefreshAfterMemberRemoved(t *testing.T) {
	s := newStand(t)
	r1 := newSession(t, s)
	exec(t, s, "DELETE FROM membership WHERE org_id = $1 AND user_id = $2", s.fx.OrgID, s.fx.MemberID)
	assert.Zero(t, countSessions(t, s))
	requireInvalidGrant(t, refresh(t, s, r1.RefreshToken))
}

// TestRevokeEndpoint (RFC 7009): a refresh token or an access token of the
// session revokes it (logout, audited as cli.logout); unknown tokens and the
// ID token answer 200 and change nothing.
func TestRevokeEndpoint(t *testing.T) {
	ctx := context.Background()
	logouts := func(t *testing.T, s stand) int {
		var n int
		require.NoError(t, s.pool.QueryRow(ctx, "SELECT count(*) FROM audit_event WHERE action = 'cli.logout'").Scan(&n))
		return n
	}
	for name, pick := range map[string]func(tokenResponse) string{
		"refresh": func(tr tokenResponse) string { return tr.RefreshToken },
		"access":  func(tr tokenResponse) string { return tr.AccessToken },
	} {
		t.Run(name, func(t *testing.T) {
			s := newStand(t)
			tr := newSession(t, s)
			for _, junk := range []string{"garbage", "wrt_" + tr.RefreshToken, tr.IDToken, s.fx.MemberID.String()} {
				assert.Equal(t, http.StatusOK, revoke(t, s, junk))
			}
			reason, _ := session(t, s)
			assert.Empty(t, reason, "unknown tokens change nothing")
			assert.Zero(t, logouts(t, s))

			assert.Equal(t, http.StatusOK, revoke(t, s, pick(tr)))
			reason, _ = session(t, s)
			assert.Equal(t, "logout", reason)
			var actor, target string
			require.NoError(t, s.pool.QueryRow(ctx, `SELECT actor_user_id::text, target_id FROM audit_event
				WHERE action = 'cli.logout' AND org_id = $1`, s.fx.OrgID).Scan(&actor, &target))
			assert.Equal(t, s.fx.MemberID.String(), actor)
			requireInvalidGrant(t, refresh(t, s, tr.RefreshToken))

			assert.Equal(t, http.StatusOK, revoke(t, s, pick(tr)), "revoking twice is not an error")
			assert.Equal(t, 1, logouts(t, s))
		})
	}
}
