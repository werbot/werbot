package app

import (
	"context"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/identity"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/session"
	"github.com/werbot/werbot/internal/tunnel"
)

// A web ticket resolves to its member with teams and the ticket's mode,
// resource, session, window and browser address, only for the organization
// and instance it was issued to and only while it lives; a removed member's
// ticket resolves to nothing.
func TestTunnelUsersByWebTicket(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := t.Context()
	a, b := dbtest.Seed(t, pool), dbtest.Seed(t, pool)
	a.OnlineInstance(t, pool, a.InstanceID, "ssh")
	team := dbtest.Team(t, pool, a.OrgID, a.MemberID)
	sessions := session.New(pool)
	users := tunnelUsers{ids: identity.New(pool, nil, identity.Config{}), sessions: sessions}

	web, err := sessions.CreateTerminalTicket(ctx, a.Principal(a.MemberID), "web", 120, 40, uuid.New())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "UPDATE stream_ticket SET client_ip = '203.0.113.7' WHERE id = $1", web.ID)
	require.NoError(t, err)
	u, wt, err := users.ByWebTicket(ctx, a.OrgID, a.InstanceID, web.ID)
	require.NoError(t, err)
	assert.Equal(t, a.MemberID, u.ID)
	assert.NotEmpty(t, u.Login)
	assert.Equal(t, []uuid.UUID{team}, u.TeamIDs)
	assert.Equal(t, tunnel.WebTicket{Mode: "web", Resource: "web", Cols: 120, Rows: 40, ClientIP: netip.MustParseAddr("203.0.113.7")}, wt)

	live := a.StartSession(t, pool, a.MemberID)
	observe, err := sessions.CreateObserveTicket(ctx, a.Principal(a.OwnerID), live, uuid.New())
	require.NoError(t, err)
	u, wt, err = users.ByWebTicket(ctx, a.OrgID, a.InstanceID, observe.ID)
	require.NoError(t, err)
	assert.Equal(t, a.OwnerID, u.ID)
	assert.Equal(t, tunnel.WebTicket{Mode: "observe", SessionID: &live}, wt)

	for name, c := range map[string][3]uuid.UUID{
		"another organization": {b.OrgID, a.InstanceID, web.ID},
		"another instance":     {a.OrgID, a.OtherInstanceID, web.ID},
		"unknown ticket":       {a.OrgID, a.InstanceID, uuid.New()},
	} {
		_, _, err := users.ByWebTicket(ctx, c[0], c[1], c[2])
		require.ErrorIs(t, err, apperr.ErrNotFound, name)
	}

	_, err = pool.Exec(ctx, "UPDATE stream_ticket SET expires_at = now() - interval '1 second' WHERE id = $1", web.ID)
	require.NoError(t, err)
	_, _, err = users.ByWebTicket(ctx, a.OrgID, a.InstanceID, web.ID)
	require.ErrorIs(t, err, apperr.ErrNotFound, "expired")

	_, err = pool.Exec(ctx, "DELETE FROM membership WHERE org_id = $1 AND user_id = $2", a.OrgID, a.OwnerID)
	require.NoError(t, err)
	_, _, err = users.ByWebTicket(ctx, a.OrgID, a.InstanceID, observe.ID)
	require.ErrorIs(t, err, apperr.ErrNotFound, "removed member")
}

// TestRelayWiring: werbot server registers its replica with the internal
// URL from the environment, serves the relay gauge on /metrics and removes
// the row when it stops.
func TestRelayWiring(t *testing.T) {
	pool, dbURL := dbtest.New(t)
	httpAddr, metricsAddr, internalAddr := freeAddr(t), freeAddr(t), freeAddr(t)
	internalURL := "https://" + internalAddr
	r := startRun(t, []string{"server"}, env(map[string]string{
		"WERBOT_DATABASE_URL": dbURL, "WERBOT_HTTP_ADDR": httpAddr, "WERBOT_PUBLIC_URL": "http://" + httpAddr,
		"WERBOT_SECRET_KEY": testSecretKey, "WERBOT_GATEWAY_ADDR": freeAddr(t), "WERBOT_DATA_DIR": t.TempDir(),
		"WERBOT_METRICS_ADDR": metricsAddr, "WERBOT_INTERNAL_ADDR": internalAddr, "WERBOT_INTERNAL_URL": internalURL,
	})).healthy(t)
	rows := func() []string {
		var urls []string
		rs, err := pool.Query(t.Context(), "SELECT internal_url FROM server_instance")
		require.NoError(t, err)
		for rs.Next() {
			var u string
			require.NoError(t, rs.Scan(&u))
			urls = append(urls, u)
		}
		require.NoError(t, rs.Err())
		return urls
	}
	require.Eventually(t, func() bool { return len(rows()) == 1 }, 10*time.Second, 50*time.Millisecond)
	assert.Equal(t, []string{internalURL}, rows())
	_, body := get(t, "http://"+metricsAddr+"/metrics")
	assert.Contains(t, body, "\nwerbot_relay_streams 0\n")
	r.stop()
	assert.Empty(t, rows(), "a stopped replica removes its row")
}

// TestRealtimeRoutesRefuseAgentTokens: an agent run token gets 403 on both
// browser stream routes and PermissionDenied from TerminateSession, an agent
// profile token 401 on the routes; none of them issues a ticket or writes
// an audit event.
func TestRealtimeRoutesRefuseAgentTokens(t *testing.T) {
	r := startAgentRig(t)
	_, _, refresh := r.newAgentProfile(t, "ops")
	run := r.runToken(t, "ops")
	profile, oauthErr := r.refreshProfile(t, refresh)
	require.Empty(t, oauthErr)
	sid := uuid.NewString()
	audit := func() int {
		var n int
		require.NoError(t, r.pool.QueryRow(t.Context(), "SELECT count(*) FROM audit_event WHERE action LIKE 'session.%'").Scan(&n))
		return n
	}
	before := audit()

	for _, path := range []string{"/api/terminal?resource=srv", "/api/sessions/" + sid + "/live"} {
		assert.Equal(t, http.StatusForbidden, upgradeStatus(t, r.base+path, run), "run token: "+path)
		assert.Equal(t, http.StatusUnauthorized, upgradeStatus(t, r.base+path, profile), "profile token: "+path)
	}
	_, err := werbotv1connect.NewSessionServiceClient(http.DefaultClient, r.base).TerminateSession(t.Context(),
		withBearer(&werbotv1.TerminateSessionRequest{Id: sid, Reason: "x"}, run))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))

	var tickets int
	require.NoError(t, r.pool.QueryRow(t.Context(), "SELECT count(*) FROM stream_ticket").Scan(&tickets))
	assert.Zero(t, tickets)
	assert.Equal(t, before, audit())
}

// upgradeStatus is the status of a WebSocket upgrade of url with the bearer
// token and the server's Origin.
func upgradeStatus(t *testing.T, url, token string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	conn, res, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(url, "http"), &websocket.DialOptions{ //nolint:bodyclose // websocket.Dial owns the body
		HTTPHeader: http.Header{"Origin": {testPublicURL}, "Authorization": {"Bearer " + token}}})
	if err == nil {
		_ = conn.CloseNow()
		return http.StatusSwitchingProtocols
	}
	require.NotNil(t, res, err)
	return res.StatusCode
}

// The tunnel's session start keeps the entry: a browser terminal is stored
// with front "web".
func TestTunnelSessionsStartKeepsFront(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	ctx, org, owner := s.org(t, "owner")
	gw, ids := s.gatewayPool(ctx, t, owner)
	var instance uuid.UUID
	require.NoError(t, s.pool.QueryRow(ctx, `INSERT INTO gateway_instance (gateway_id, org_id, hostname, cert_serial, cert_expires_at)
		VALUES ($1, $2, 'h', 'ab', now() + interval '1 hour') RETURNING id`, gw, org).Scan(&instance))
	sessions := newTunnelDeps(s.d, metrics.NewRegistry(), slog.New(slog.DiscardHandler)).Sessions
	id, err := sessions.Start(context.Background(), tunnel.SessionStart{OrgID: org, UserID: owner, GatewayID: gw, InstanceID: instance,
		ResourceID: uuid.MustParse(ids[0]), ClientIP: netip.MustParseAddr("203.0.113.7"), Kind: "terminal", Front: "web"})
	require.NoError(t, err)
	var front string
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT front FROM session WHERE id = $1`, id).Scan(&front))
	assert.Equal(t, "web", front)
}
