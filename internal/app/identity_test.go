package app

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"slices"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/webui"
)

const ownerPassword = "correct horse battery"

var ownerSetup = &werbotv1.SetupRequest{
	Email: "owner@example.com", Login: "owner", Name: "Owner", Password: ownerPassword, OrganizationName: "Acme",
}

var loopback = []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}

// headerTransport adds one header to every request, like a load balancer in front of web.
type headerTransport struct{ key, value string }

func (h headerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set(h.key, h.value)
	return http.DefaultTransport.RoundTrip(r)
}

// jarClient is an http.Client keeping cookies like a browser.
func jarClient(t *testing.T, tr http.RoundTripper) (*http.Client, http.CookieJar) {
	t.Helper()
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	return &http.Client{Jar: jar, Transport: tr}, jar
}

func TestServeAuthFlow(t *testing.T) {
	pool, _ := dbtest.New(t)
	// development: cookie without Secure, which the jar sends over plain http.
	cfg := config.Config{HTTPAddr: freeAddr(t), Env: "development", WebEmbedded: true}
	startServe(t, cfg, pool, nil, fstest.MapFS{})
	hc, _ := jarClient(t, http.DefaultTransport)
	auth := werbotv1connect.NewAuthServiceClient(hc, "http://"+cfg.HTTPAddr)
	ctx := context.Background()

	state, err := auth.GetSetupState(ctx, connect.NewRequest(&werbotv1.GetSetupStateRequest{}))
	require.NoError(t, err)
	assert.True(t, state.Msg.GetNeedsSetup())

	bad := proto.CloneOf(ownerSetup)
	bad.Email = "nope"
	_, err = auth.Setup(ctx, connect.NewRequest(bad))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "protovalidate interceptor")

	_, err = auth.Setup(ctx, connect.NewRequest(ownerSetup))
	require.NoError(t, err)
	sess, err := auth.GetSession(ctx, connect.NewRequest(&werbotv1.GetSessionRequest{}))
	require.NoError(t, err)
	assert.Equal(t, "owner@example.com", sess.Msg.GetUser().GetEmail())

	_, err = auth.Logout(ctx, connect.NewRequest(&werbotv1.LogoutRequest{}))
	require.NoError(t, err)
	_, err = auth.GetSession(ctx, connect.NewRequest(&werbotv1.GetSessionRequest{}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

// webClient starts werbot web (APIURL = server at apiAddr, trusting loopback as
// its load balancer) and returns an AuthService client that reaches the server
// only through web, sending X-Forwarded-For: 198.51.100.7.
func webClient(t *testing.T, apiAddr string) (werbotv1connect.AuthServiceClient, http.CookieJar, *url.URL) {
	t.Helper()
	var draining atomic.Bool
	h, err := webui.Handler(webui.Config{APIURL: "http://" + apiAddr, TrustedProxies: loopback},
		fstest.MapFS{}, &draining, metrics.NewRegistry(), slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	web := httptest.NewServer(h)
	t.Cleanup(web.Close)
	hc, jar := jarClient(t, headerTransport{"X-Forwarded-For", "198.51.100.7"})
	u, err := url.Parse(web.URL)
	require.NoError(t, err)
	return werbotv1connect.NewAuthServiceClient(hc, web.URL), jar, u
}

func latestSessionIP(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var ip string
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT host(ip) FROM user_session ORDER BY created_at DESC LIMIT 1").Scan(&ip))
	return ip
}

func TestWebLoginThroughProxy(t *testing.T) {
	ctx := context.Background()
	login := &werbotv1.LoginRequest{Email: ownerSetup.GetEmail(), Password: ownerPassword}

	pool, _ := dbtest.New(t)
	cfg := config.Config{HTTPAddr: freeAddr(t), Env: "development", WebEmbedded: false, TrustedProxies: loopback}
	startServe(t, cfg, pool, nil, fstest.MapFS{})
	auth, jar, webURL := webClient(t, cfg.HTTPAddr)

	_, err := auth.Setup(ctx, connect.NewRequest(ownerSetup))
	require.NoError(t, err)
	_, err = auth.Logout(ctx, connect.NewRequest(&werbotv1.LogoutRequest{}))
	require.NoError(t, err)
	res, err := auth.Login(ctx, connect.NewRequest(login))
	require.NoError(t, err)
	c, err := http.ParseSetCookie(res.Header().Get("Set-Cookie"))
	require.NoError(t, err)
	assert.Equal(t, "werbot_session", c.Name)
	assert.Empty(t, c.Domain)
	assert.True(t, slices.ContainsFunc(jar.Cookies(webURL), func(k *http.Cookie) bool {
		return k.Name == "werbot_session" && k.Value == c.Value
	}), "jar keeps the cookie for web's host")

	_, err = auth.GetSession(ctx, connect.NewRequest(&werbotv1.GetSessionRequest{}))
	require.NoError(t, err, "the cookie of web's host is accepted by server")

	assert.Equal(t, "198.51.100.7", latestSessionIP(t, pool))
	var auditIP string
	require.NoError(t, pool.QueryRow(ctx, "SELECT host(ip) FROM audit_event WHERE action = 'auth.login'").Scan(&auditIP))
	assert.Equal(t, "198.51.100.7", auditIP)

	// Server without web's subnet in WERBOT_TRUSTED_PROXIES sees web's own address.
	pool2, _ := dbtest.New(t)
	cfg2 := config.Config{HTTPAddr: freeAddr(t), Env: "development", WebEmbedded: false}
	startServe(t, cfg2, pool2, nil, fstest.MapFS{})
	auth2, _, _ := webClient(t, cfg2.HTTPAddr)
	_, err = auth2.Setup(ctx, connect.NewRequest(ownerSetup))
	require.NoError(t, err)
	_, err = auth2.Login(ctx, connect.NewRequest(login))
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", latestSessionIP(t, pool2))
}

func TestLogInternalErrors(t *testing.T) {
	var logs bytes.Buffer
	ic := logInternalErrors(slog.New(slog.NewTextHandler(&logs, nil)))
	call := func(err error) {
		_, _ = ic(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) { return nil, err })(
			context.Background(), connect.NewRequest(&werbotv1.GetSessionRequest{}))
	}

	call(apperr.ToConnect(apperr.ErrNotFound))
	assert.Empty(t, logs.String(), "only CodeInternal is logged")
	call(apperr.ToConnect(errors.New("db exploded")))
	assert.Contains(t, logs.String(), "rpc failed")
	assert.Contains(t, logs.String(), "db exploded")
}
