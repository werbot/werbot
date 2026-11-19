package oauth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/identity/oauth"
	"github.com/werbot/werbot/internal/platform/apperr"
)

type deviceResponse struct {
	Status                  int    `json:"-"`
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
	Error                   string `json:"error"`
}

func postJSON(t *testing.T, u string, form url.Values, out any) int {
	t.Helper()
	res, err := http.PostForm(u, form) //nolint:gosec // loopback test server
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	require.NoError(t, json.NewDecoder(res.Body).Decode(out))
	return res.StatusCode
}

// deviceAuthorize starts a device flow on srv with scope.
func deviceAuthorize(t *testing.T, srv *httptest.Server, scope string) deviceResponse {
	t.Helper()
	var dr deviceResponse
	dr.Status = postJSON(t, srv.URL+"/oauth/device_authorization", url.Values{
		"client_id": {oauth.ClientCLI}, "scope": {scope}, "werbot_hostname": {"build-01"},
	}, &dr)
	return dr
}

// startDevice starts a device flow with the scopes werbot login asks for.
func startDevice(t *testing.T, srv *httptest.Server) deviceResponse {
	t.Helper()
	dr := deviceAuthorize(t, srv, "openid offline_access")
	require.Equal(t, http.StatusOK, dr.Status, dr.Error)
	return dr
}

// poll asks srv for the tokens of deviceCode once.
func poll(t *testing.T, srv *httptest.Server, deviceCode string) tokenResponse {
	t.Helper()
	var tr tokenResponse
	tr.Status = postJSON(t, srv.URL+"/oauth/token", url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}, "client_id": {oauth.ClientCLI}, "device_code": {deviceCode},
	}, &tr)
	return tr
}

func deviceStatus(t *testing.T, s stand, userCode string) string {
	t.Helper()
	var st string
	require.NoError(t, s.pool.QueryRow(context.Background(), "SELECT status FROM oauth_device_code WHERE user_code = $1", userCode).Scan(&st))
	return st
}

// TestDeviceAuthorization: RFC 8628 response; only the device code's hash is stored.
func TestDeviceAuthorization(t *testing.T) {
	s := newStand(t)
	dr := startDevice(t, s.srv)
	assert.Regexp(t, `^[BCDFGHJKLMNPQRSTVWXZ]{4}-[BCDFGHJKLMNPQRSTVWXZ]{4}$`, dr.UserCode)
	assert.Equal(t, testWebURL+"/device", dr.VerificationURI)
	assert.Contains(t, dr.VerificationURIComplete, "user_code="+dr.UserCode)
	assert.Equal(t, 600, dr.ExpiresIn)
	assert.Equal(t, 5, dr.Interval)
	require.NotEmpty(t, dr.DeviceCode)

	var (
		row, host, ip, status string
		scopes                []string
	)
	require.NoError(t, s.pool.QueryRow(context.Background(), `SELECT row_to_json(d)::text, client_hostname, host(client_ip), status, scopes
		FROM oauth_device_code d WHERE device_code_hash = sha256(convert_to($1, 'UTF8'))`, dr.DeviceCode).Scan(&row, &host, &ip, &status, &scopes))
	assert.NotContains(t, row, dr.DeviceCode, "only the hash is stored")
	assert.Equal(t, "build-01", host)
	assert.Equal(t, "127.0.0.1", ip)
	assert.Equal(t, "pending", status)
	assert.Equal(t, []string{"openid", "offline_access"}, scopes)
}

// TestDeviceRequiresOfflineAccess: the CLI lives on its refresh token.
func TestDeviceRequiresOfflineAccess(t *testing.T) {
	s := newStand(t)
	dr := deviceAuthorize(t, s.srv, "openid")
	assert.Equal(t, http.StatusBadRequest, dr.Status)
	assert.Equal(t, "invalid_scope", dr.Error)
	var n int
	require.NoError(t, s.pool.QueryRow(context.Background(), "SELECT count(*) FROM oauth_device_code").Scan(&n))
	assert.Zero(t, n)
}

// TestDevicePollPending: before consent the CLI keeps polling.
func TestDevicePollPending(t *testing.T) {
	s := newStand(t)
	dr := startDevice(t, s.srv)
	tr := poll(t, s.srv, dr.DeviceCode)
	assert.Equal(t, http.StatusBadRequest, tr.Status)
	assert.Equal(t, "authorization_pending", tr.Error)
	var polled bool
	require.NoError(t, s.pool.QueryRow(context.Background(), "SELECT last_poll_at IS NOT NULL FROM oauth_device_code WHERE user_code = $1", dr.UserCode).Scan(&polled))
	assert.True(t, polled)
	assert.Equal(t, "access_denied", poll(t, s.srv, "unknown-code").Error)
}

// TestDeviceApproveFlow: a typed user code (any case, spaces) is found and
// approved; the next poll gets the tokens of a cli_session, audited as
// cli.login with flow device.
func TestDeviceApproveFlow(t *testing.T) {
	s := newStand(t)
	ctx := context.Background()
	dr := startDevice(t, s.srv)
	typed := " " + strings.ToLower(strings.Replace(dr.UserCode, "-", " ", 1)) + " "
	info, err := s.p.LookupDeviceCode(ctx, cookieAs(s.fx, s.fx.MemberID), typed)
	require.NoError(t, err)
	assert.Equal(t, oauth.ClientCLI, info.ClientID)
	assert.Equal(t, "build-01", info.Hostname)
	assert.Equal(t, "127.0.0.1", info.IP.String())
	assert.Equal(t, []string{"openid", "offline_access"}, info.Scopes)
	require.NoError(t, s.p.ApproveDeviceCode(ctx, cookieAs(s.fx, s.fx.MemberID), typed))
	_, err = s.p.LookupDeviceCode(ctx, cookieAs(s.fx, s.fx.MemberID), typed)
	require.ErrorIs(t, err, apperr.ErrNotFound, "approved: no longer pending")

	tr := poll(t, s.srv, dr.DeviceCode)
	require.Equal(t, http.StatusOK, tr.Status, tr.Error)
	assert.Regexp(t, `^wrt_[A-Za-z0-9_-]{43}$`, tr.RefreshToken)
	assert.Equal(t, "Bearer", tr.TokenType)
	assert.InDelta(t, 600, tr.ExpiresIn, 2)
	claims, err := s.p.VerifyAccessToken(ctx, tr.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, s.fx.MemberID, claims.UserID)
	assert.Equal(t, s.fx.OrgID, claims.OrgID)

	var host, ip string
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT hostname, host(ip) FROM cli_session WHERE id = $1 AND org_id = $2 AND user_id = $3",
		claims.CLISessionID, s.fx.OrgID, s.fx.MemberID).Scan(&host, &ip))
	assert.Equal(t, "build-01", host)
	assert.Equal(t, "127.0.0.1", ip)
	var data map[string]any
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT data FROM audit_event WHERE action = 'cli.login' AND org_id = $1
		AND actor_user_id = $2 AND target_id = $3`, s.fx.OrgID, s.fx.MemberID, claims.CLISessionID.String()).Scan(&data))
	assert.Equal(t, map[string]any{"client_id": oauth.ClientCLI, "hostname": "build-01", "flow": "device"}, data)
	assert.Equal(t, "used", deviceStatus(t, s, dr.UserCode))
}

// TestDeviceCodeUsedOnce: approval on replica B, five parallel polls on A:
// one cli_session; the code is spent afterwards.
func TestDeviceCodeUsedOnce(t *testing.T) {
	s := newStand(t)
	b, _ := serveProvider(t, s.pool, s.key, s.srv.URL)
	dr := startDevice(t, s.srv)
	require.NoError(t, b.ApproveDeviceCode(context.Background(), cookieAs(s.fx, s.fx.MemberID), dr.UserCode))
	const n = 5
	results := make([]tokenResponse, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { results[i] = poll(t, s.srv, dr.DeviceCode) })
	}
	wg.Wait()
	ok := 0
	for _, r := range results {
		if r.Status == http.StatusOK {
			ok++
			continue
		}
		assert.Contains(t, []string{"invalid_grant", "expired_token"}, r.Error)
	}
	assert.Equal(t, 1, ok)
	assert.Equal(t, 1, countSessions(t, s))
	assert.Equal(t, "expired_token", poll(t, s.srv, dr.DeviceCode).Error)
	assert.Equal(t, 1, countSessions(t, s))
}

// TestDeviceDenied: a denied code answers access_denied and is gone from consent.
func TestDeviceDenied(t *testing.T) {
	s := newStand(t)
	ctx := context.Background()
	dr := startDevice(t, s.srv)
	require.NoError(t, s.p.DenyDeviceCode(ctx, cookieAs(s.fx, s.fx.MemberID), dr.UserCode))
	assert.Equal(t, "access_denied", poll(t, s.srv, dr.DeviceCode).Error)
	_, err := s.p.LookupDeviceCode(ctx, cookieAs(s.fx, s.fx.MemberID), dr.UserCode)
	require.ErrorIs(t, err, apperr.ErrNotFound)
	require.ErrorIs(t, s.p.ApproveDeviceCode(ctx, cookieAs(s.fx, s.fx.MemberID), dr.UserCode), apperr.ErrNotFound)
	require.ErrorIs(t, s.p.DenyDeviceCode(ctx, cookieAs(s.fx, s.fx.MemberID), "BCDF-GHJK"), apperr.ErrNotFound)
	assert.Zero(t, countSessions(t, s))
}

// TestDeviceExpired: an ended code is neither approved nor exchanged, even
// when it was approved before it ended.
func TestDeviceExpired(t *testing.T) {
	s := newStand(t)
	ctx := context.Background()
	expire := func(code string) {
		_, err := s.pool.Exec(ctx, "UPDATE oauth_device_code SET expires_at = now() - interval '1 second' WHERE user_code = $1", code)
		require.NoError(t, err)
	}
	dr := startDevice(t, s.srv)
	expire(dr.UserCode)
	assert.Equal(t, "expired_token", poll(t, s.srv, dr.DeviceCode).Error)
	require.ErrorIs(t, s.p.ApproveDeviceCode(ctx, cookieAs(s.fx, s.fx.MemberID), dr.UserCode), apperr.ErrNotFound)

	approved := startDevice(t, s.srv)
	require.NoError(t, s.p.ApproveDeviceCode(ctx, cookieAs(s.fx, s.fx.MemberID), approved.UserCode))
	expire(approved.UserCode)
	assert.Equal(t, "expired_token", poll(t, s.srv, approved.DeviceCode).Error)
	assert.Zero(t, countSessions(t, s))
}

// TestDeviceApproveNeedsMembership: only a member of the caller's
// organization approves.
func TestDeviceApproveNeedsMembership(t *testing.T) {
	s := newStand(t)
	dr := startDevice(t, s.srv)
	stranger := cookieAs(s.fx, s.fx.MemberID)
	_, err := s.pool.Exec(context.Background(), "DELETE FROM membership WHERE org_id = $1 AND user_id = $2", s.fx.OrgID, s.fx.MemberID)
	require.NoError(t, err)
	require.ErrorIs(t, s.p.ApproveDeviceCode(context.Background(), stranger, dr.UserCode), apperr.ErrNotFound)
	assert.Equal(t, "pending", deviceStatus(t, s, dr.UserCode))
}
