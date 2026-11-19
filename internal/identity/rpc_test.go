package identity

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/principal"
)

// newAuthClient serves the AuthService behind Middleware, as serve does.
func newAuthClient(t *testing.T, svc *Service) werbotv1connect.AuthServiceClient {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(werbotv1connect.NewAuthServiceHandler(NewHandler(svc)))
	srv := httptest.NewServer(Middleware(svc, nil)(mux))
	t.Cleanup(srv.Close)
	return werbotv1connect.NewAuthServiceClient(srv.Client(), srv.URL)
}

// withCookie returns a request carrying cookie name=value (set by hand: Secure cookies
// are not sent over the plain-http test server by a cookie jar).
func withCookie[T any](msg *T, name, value string) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set("Cookie", name+"="+value)
	return req
}

func login(t *testing.T, client werbotv1connect.AuthServiceClient) *http.Cookie {
	t.Helper()
	res, err := client.Login(context.Background(), connect.NewRequest(&werbotv1.LoginRequest{Email: "alice@example.com", Password: testPassword}))
	require.NoError(t, err)
	c, err := http.ParseSetCookie(res.Header().Get("Set-Cookie"))
	require.NoError(t, err)
	return c
}

func TestLoginCookieFlags(t *testing.T) {
	for _, secure := range []bool{true, false} {
		svc, pool, _ := newTestService(t)
		svc.cfg.SecureCookies = secure
		seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleOwner)
		client := newAuthClient(t, svc)

		c := login(t, client)
		name := "werbot_session"
		if secure {
			name = "__Host-werbot_session"
		}
		assert.Equal(t, name, c.Name)
		assert.Empty(t, c.Domain)
		assert.True(t, c.HttpOnly)
		assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
		assert.Equal(t, "/", c.Path)
		assert.Equal(t, 604800, c.MaxAge)
		assert.Equal(t, secure, c.Secure)
		assert.Len(t, c.Value, 43)

		if secure {
			// The development cookie name is ignored when cookies are secure.
			_, err := client.GetSession(context.Background(), withCookie(&werbotv1.GetSessionRequest{}, "werbot_session", c.Value))
			assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
		}
	}
}

func TestGetSessionRequiresCookie(t *testing.T) {
	svc, pool, _ := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleOwner)
	client := newAuthClient(t, svc)

	_, err := client.GetSession(context.Background(), connect.NewRequest(&werbotv1.GetSessionRequest{}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))

	c := login(t, client)
	res, err := client.GetSession(context.Background(), withCookie(&werbotv1.GetSessionRequest{}, c.Name, c.Value))
	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", res.Msg.GetUser().GetEmail())
	assert.Equal(t, "Acme", res.Msg.GetOrganization().GetName())
	assert.Equal(t, werbotv1.Role_ROLE_OWNER, res.Msg.GetRole())
}

func TestLogoutClearsCookie(t *testing.T) {
	svc, pool, _ := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleOwner)
	client := newAuthClient(t, svc)
	c := login(t, client)

	res, err := client.Logout(context.Background(), withCookie(&werbotv1.LogoutRequest{}, c.Name, c.Value))
	require.NoError(t, err)
	cleared, err := http.ParseSetCookie(res.Header().Get("Set-Cookie"))
	require.NoError(t, err)
	assert.Equal(t, c.Name, cleared.Name)
	assert.Less(t, cleared.MaxAge, 0)

	_, err = client.GetSession(context.Background(), withCookie(&werbotv1.GetSessionRequest{}, c.Name, c.Value))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestMiddlewareExtendsStaleSession(t *testing.T) {
	svc, pool, _ := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleOwner)
	client := newAuthClient(t, svc)
	c := login(t, client)

	res, err := client.GetSession(context.Background(), withCookie(&werbotv1.GetSessionRequest{}, c.Name, c.Value))
	require.NoError(t, err)
	assert.Empty(t, res.Header().Values("Set-Cookie"), "fresh session: cookie not re-set")

	_, err = pool.Exec(context.Background(), "UPDATE user_session SET last_seen_at = now() - interval '2 minutes'")
	require.NoError(t, err)
	res, err = client.GetSession(context.Background(), withCookie(&werbotv1.GetSessionRequest{}, c.Name, c.Value))
	require.NoError(t, err)
	again, err := http.ParseSetCookie(res.Header().Get("Set-Cookie"))
	require.NoError(t, err)
	assert.Equal(t, c.Value, again.Value)
	assert.Equal(t, 604800, again.MaxAge)
}

func TestMiddlewareStoreFailureIsUnavailable(t *testing.T) {
	svc, pool, _ := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleOwner)
	var logs bytes.Buffer
	svc.cfg.Logger = slog.New(slog.NewTextHandler(&logs, nil))
	mux := http.NewServeMux()
	mux.Handle(werbotv1connect.NewAuthServiceHandler(NewHandler(svc)))
	mux.HandleFunc("/", func(_ http.ResponseWriter, r *http.Request) {
		_, ok := principal.From(r.Context())
		assert.False(t, ok, "SPA request goes on without a subject")
	})
	srv := httptest.NewServer(Middleware(svc, nil)(mux))
	t.Cleanup(srv.Close)
	client := werbotv1connect.NewAuthServiceClient(srv.Client(), srv.URL)
	c := login(t, client)
	pool.Close() // the session lookup now fails with a store error, not "no session"

	_, err := client.GetSession(context.Background(), withCookie(&werbotv1.GetSessionRequest{}, c.Name, c.Value))
	assert.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/werbot.v1.AuthService/GetSession", strings.NewReader("{}"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(c)
	res, err := srv.Client().Do(req)
	require.NoError(t, err)
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.NoError(t, res.Body.Close())
	assert.Equal(t, http.StatusServiceUnavailable, res.StatusCode)
	assert.JSONEq(t, `{"code":"unavailable","message":"unavailable"}`, string(body))

	req, err = http.NewRequest(http.MethodGet, srv.URL+"/invite/one-time-token", nil)
	require.NoError(t, err)
	req.AddCookie(c)
	res, err = srv.Client().Do(req)
	require.NoError(t, err)
	require.NoError(t, res.Body.Close())
	assert.Equal(t, http.StatusOK, res.StatusCode)

	assert.Contains(t, logs.String(), "session lookup failed")
	assert.NotContains(t, logs.String(), c.Value, "the cookie never reaches the log")
	assert.NotContains(t, logs.String(), "one-time-token", "SPA paths carry tokens: the path is not logged")
}

// publicMethods are the only RPCs served without a session.
var publicMethods = map[string]bool{
	"GetSetupState": true, "Setup": true, "Login": true,
	"RequestPasswordReset": true, "ResetPassword": true, "AcceptInvite": true,
}

func TestRPCRequiresSession(t *testing.T) {
	svc, _, _ := newTestService(t)
	h := NewHandler(svc)
	mux := http.NewServeMux()
	mux.Handle(werbotv1connect.NewAuthServiceHandler(h))
	mux.Handle(werbotv1connect.NewProfileServiceHandler(h))
	mux.Handle(werbotv1connect.NewSSHKeyServiceHandler(h))
	mux.Handle(werbotv1connect.NewMemberServiceHandler(h))
	mux.Handle(werbotv1connect.NewTeamServiceHandler(h))
	mux.Handle(werbotv1connect.NewCLISessionServiceHandler(h))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	checked := 0
	for _, name := range []string{
		werbotv1connect.AuthServiceName, werbotv1connect.ProfileServiceName, werbotv1connect.SSHKeyServiceName,
		werbotv1connect.MemberServiceName, werbotv1connect.TeamServiceName, werbotv1connect.CLISessionServiceName,
	} {
		d, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(name))
		require.NoError(t, err)
		methods := d.(protoreflect.ServiceDescriptor).Methods()
		for i := range methods.Len() {
			m := methods.Get(i)
			if publicMethods[string(m.Name())] {
				continue
			}
			res, err := srv.Client().Post(srv.URL+"/"+name+"/"+string(m.Name()), "application/json", strings.NewReader("{}"))
			require.NoError(t, err)
			require.NoError(t, res.Body.Close())
			assert.Equal(t, http.StatusUnauthorized, res.StatusCode, "%s/%s", name, m.Name())
			checked++
		}
	}
	assert.Equal(t, 23, checked)
}
