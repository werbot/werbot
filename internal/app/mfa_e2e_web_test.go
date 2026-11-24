//go:build unix

package app

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
)

// dialWeb opens the WebSocket path as the browser session c, with the web
// origin, and fails the test unless the upgrade succeeds.
func (m *mfaStack) dialWeb(t *testing.T, c mfaClients, path string) *wsConn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), wsWait)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(m.url, "http")+path, &websocket.DialOptions{ //nolint:bodyclose // websocket.Dial owns the body
		HTTPClient: c.hc, HTTPHeader: http.Header{"Origin": {m.webURL}}})
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.CloseNow() })
	w := &wsConn{t: t, conn: conn, closed: make(chan struct{})}
	go w.collect()
	return w
}

// stepUp passes a fresh second factor in the session c with a TOTP code.
func stepUp(t *testing.T, c mfaClients, code string) {
	t.Helper()
	_, err := c.auth.CompleteStepUp(context.Background(), connect.NewRequest(&werbotv1.CompleteStepUpRequest{
		Proof: &werbotv1.CompleteStepUpRequest_TotpCode{TotpCode: code}}))
	require.NoError(t, err)
}

// A session whose second factor is 6 min old cannot change the gateways; a
// step-up with a TOTP code makes it fresh again.
func TestE2EAdminActionNeedsFreshMFA(t *testing.T) {
	m := startMFAStack(t)
	ctx := context.Background()
	secret := enrollTOTP(t, m.web)

	hc, _ := jarClient(t, http.DefaultTransport)
	c := newMFAClients(hc, m.url)
	res, err := c.auth.Login(ctx, connect.NewRequest(&werbotv1.LoginRequest{Email: "alice@example.com", Password: ownerPassword}))
	require.NoError(t, err)
	require.True(t, res.Msg.GetMfaRequired())
	_, err = c.auth.CompleteMFA(ctx, connect.NewRequest(&werbotv1.CompleteMFARequest{MfaToken: res.Msg.GetMfaToken(),
		Proof: &werbotv1.CompleteMFARequest_TotpCode{TotpCode: totpAt(t, secret, 0)}}))
	require.NoError(t, err)
	_, err = m.pool.Exec(ctx, "UPDATE user_session SET mfa_at = now() - interval '6 minutes' WHERE mfa_at IS NOT NULL")
	require.NoError(t, err)

	gws := werbotv1connect.NewGatewayServiceClient(hc, m.url)
	create := func() error {
		_, err := gws.CreateGateway(ctx, connect.NewRequest(&werbotv1.CreateGatewayRequest{Name: "edge"}))
		return err
	}
	assertDenied(t, create(), "mfa required", "6 min after the factor")
	stepUp(t, c, totpAt(t, secret, 30*time.Second))
	require.NoError(t, create(), "after the step-up")
}

// A web terminal to a resource under an MFA rule closes 4001 mfa_required
// while the session has no fresh factor; after a step-up it opens a shell.
func TestE2EWebTerminalMFA(t *testing.T) {
	m := mfaSSH(t)
	secret := enrollTOTP(t, m.web)

	code, reason := m.dialWeb(t, m.web, "/api/terminal?resource=srv&cols=100&rows=30").closeStatus()
	assert.Equal(t, websocket.StatusCode(4001), code)
	assert.Equal(t, "mfa_required", reason)

	stepUp(t, m.web, totpAt(t, secret, 0))
	w := m.dialWeb(t, m.web, "/api/terminal?resource=srv&cols=100&rows=30")
	w.send("echo hi\r")
	w.expect("echo hi")
}
