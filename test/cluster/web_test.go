//go:build cluster

package cluster

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"

	sshv1 "github.com/werbot/werbot/gen/werbot/connector/ssh/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

const (
	upstreamErrors = "werbot_web_upstream_errors_total"
	unavailable    = `{"code":"unavailable","message":"api unavailable"}`
	resetLink      = "http://" + UIHost + "/reset-password/" // WERBOT_WEB_URL, not WERBOT_PUBLIC_URL
)

// get is GET url with Accept: text/html; it returns the status, headers and body.
func get(t testing.TB, api API, url string) (int, http.Header, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/html")
	res, err := api.HTTP.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	return res.StatusCode, res.Header, string(body)
}

// webErrors is werbot_web_upstream_errors_total summed over web1 and web2.
func webErrors(t testing.TB) int64 {
	t.Helper()
	return Metric(t, st.Addr(t, "web1", 9100), upstreamErrors) + Metric(t, st.Addr(t, "web2", 9100), upstreamErrors)
}

// assertIndex checks that the UI host serves the SPA index page with a CSP.
func assertIndex(t testing.TB, ui API) {
	t.Helper()
	code, h, body := get(t, ui, ui.BaseURL+"/")
	assert.Equal(t, http.StatusOK, code, "GET UI/")
	assert.Contains(t, strings.ToLower(body), "<html", "index.html")
	assert.NotEmpty(t, h.Get("Content-Security-Policy"), "UI/ has a CSP")
}

// uiResource creates web-target through the UI, sets its password and binds alice to it.
func uiResource(t *testing.T, ui API) {
	t.Helper()
	ctx := t.Context()
	cfg, err := anypb.New(&sshv1.Config{Host: "target", Port: 2222, Login: "werbot"})
	require.NoError(t, err)
	res, err := ui.Resources.CreateResource(ctx, connect.NewRequest(&werbotv1.CreateResourceRequest{
		Name: "web-target", Type: "ssh", GatewayId: st.PoolID.String(), Config: cfg,
	}))
	require.NoError(t, err)
	id := res.Msg.GetResource().GetId()
	t.Cleanup(func() { // Edge: the UI is down while a scenario step fails
		_, err := st.Edge.Resources.DeleteResource(context.Background(), connect.NewRequest(&werbotv1.DeleteResourceRequest{Id: id}))
		assert.NoError(t, err)
	})
	_, err = ui.Resources.SetCredential(ctx, connect.NewRequest(&werbotv1.SetCredentialRequest{ResourceId: id, Kind: "password", Value: []byte("werbot-target")}))
	require.NoError(t, err)
	_, err = ui.Access.CreateBinding(ctx, connect.NewRequest(&werbotv1.CreateBindingRequest{SubjectType: "user", SubjectId: st.AliceID.String(), ResourceId: id}))
	require.NoError(t, err)
}

// resetMailLogged reports whether a server logged the reset mail with the UI link.
func resetMailLogged() bool {
	out, err := st.Compose(context.Background(), "logs", "--no-color", "server1", "server2")
	if err != nil {
		return false
	}
	for line := range strings.SplitSeq(out, "\n") {
		if strings.Contains(line, "email (log mailer)") && strings.Contains(line, resetLink) {
			return true
		}
	}
	return false
}

// TestScenario13SeparateWeb: the UI goes through werbot web replicas on its own
// host (login, resources, replay, reset mail, client IP), survives one stopped
// server and answers 503 unavailable, with /readyz still 200, when both are down.
func TestScenario13SeparateWeb(t *testing.T) {
	st.Healthy(t)
	ctx := t.Context()

	for _, svc := range []string{"server1", "server2"} {
		_, err := st.Compose(ctx, "port", svc, "8080")
		assert.Error(t, err, "server HTTP of %s is not published", svc)
	}
	code, _, _ := get(t, st.Edge, st.Edge.BaseURL+"/")
	assert.Equal(t, http.StatusNotFound, code, "GET Edge/ with WERBOT_WEB_EMBEDDED=false")
	assertIndex(t, st.UI)

	res, err := st.UI.Auth.Login(ctx, connect.NewRequest(&werbotv1.LoginRequest{Email: "alice@example.com", Password: "cluster-password-42"}))
	require.NoError(t, err)
	var cookie string
	for _, line := range res.Header().Values("Set-Cookie") {
		if c, err := http.ParseSetCookie(line); err == nil && c.Name == sessionCookie {
			cookie = c.Value
			assert.Empty(t, c.Domain, "host-only cookie")
		}
	}
	require.NotEmpty(t, cookie, "werbot_session")
	ui := st.NewAPI(st.UI.BaseURL, cookie)

	var auditIP string
	require.NoError(t, st.DB.QueryRow(ctx, `SELECT host(ip) FROM audit_event WHERE action = 'auth.login' ORDER BY created_at DESC LIMIT 1`).Scan(&auditIP))
	ip, err := st.Edge.System.GetClientIP(ctx, connect.NewRequest(&werbotv1.GetClientIPRequest{}))
	require.NoError(t, err)
	assert.NotContains(t, []string{IPNginx, IPWeb1, IPWeb2}, auditIP, "audit IP is the client, not a proxy")
	assert.Equal(t, ip.Msg.GetIp(), auditIP, "audit IP of the UI login")

	uiResource(t, ui)
	bound := func() bool {
		addr, err := st.port(context.Background(), "nginx", 2201)
		if err != nil {
			return false
		}
		c, err := st.dial(addr, "alice+web-target")
		if err != nil {
			return false
		}
		defer c.Close()
		out, code, err := run(c, "hostname")
		return err == nil && code == 0 && out == "target\n"
	}
	require.Eventually(t, bound, BindingPropagation, 100*time.Millisecond, "binding made through the UI reaches the gateway")
	sh := OpenShell(t, st.Dial(t, st.Addr(t, "nginx", 2201), "alice+web-target"))
	sh.Mark(t, "via-web")
	id := st.OpenSessionOn(t, "gw1")
	sh.Exit(t)
	st.WaitVerified(t, id, UploadDrain)
	code, h, body := get(t, ui, ui.BaseURL+"/api/recordings/"+id.String()+".cast")
	assert.Equal(t, http.StatusOK, code, "replay through the UI")
	assert.Equal(t, "true", h.Get("X-Werbot-Recording-Verified"))
	assert.Contains(t, body, "via-web")

	_, err = ui.Auth.RequestPasswordReset(ctx, connect.NewRequest(&werbotv1.RequestPasswordResetRequest{Email: "alice@example.com"}))
	require.NoError(t, err)
	assert.Eventually(t, resetMailLogged, 10*time.Second, 500*time.Millisecond, "reset mail with "+resetLink)

	t.Cleanup(func() { st.Healthy(t) }) // runs before the resource delete: Edge needs the servers
	oneReplica(t, ui)
	bothDown(t, ui, id.String())
}

// oneReplica stops server1: the UI keeps working through server2.
func oneReplica(t *testing.T, ui API) {
	t.Helper()
	t0 := time.Now() // the window includes the docker CLI time of the stop
	_, err := st.Compose(t.Context(), "stop", "server1")
	require.NoError(t, err)
	ok := func() bool {
		ctx := context.Background()
		_, err1 := ui.Auth.GetSession(ctx, connect.NewRequest(&werbotv1.GetSessionRequest{}))
		_, err2 := ui.System.GetVersion(ctx, connect.NewRequest(&werbotv1.GetVersionRequest{}))
		return err1 == nil && err2 == nil
	}
	require.Eventually(t, ok, ReconnectMax-time.Since(t0), 100*time.Millisecond, "UI works on server2 alone")
}

// bothDown stops server2 too: 503 unavailable on API calls, /readyz and the SPA stay up.
func bothDown(t *testing.T, ui API, sessionID string) {
	t.Helper()
	_, err := st.Compose(t.Context(), "stop", "server2")
	require.NoError(t, err)
	e0 := webErrors(t)

	res, err := ui.HTTP.Post(ui.BaseURL+"/werbot.v1.SystemService/GetVersion", "application/json", strings.NewReader("{}"))
	require.NoError(t, err)
	body, err := io.ReadAll(res.Body)
	_ = res.Body.Close()
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, res.StatusCode)
	assert.Equal(t, "application/json", res.Header.Get("Content-Type"))
	assert.Equal(t, unavailable, strings.TrimSpace(string(body)))

	_, err = ui.System.GetVersion(t.Context(), connect.NewRequest(&werbotv1.GetVersionRequest{}))
	assert.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))
	code, _, _ := get(t, ui, ui.BaseURL+"/api/recordings/"+sessionID+".cast")
	assert.Equal(t, http.StatusServiceUnavailable, code, "replay without servers")

	assert.Equal(t, http.StatusOK, Ready(st.Addr(t, "web1", 8090)), "web1 /readyz")
	assert.Equal(t, http.StatusOK, Ready(st.Addr(t, "web2", 8090)), "web2 /readyz")
	assertIndex(t, ui)
	assert.Greater(t, webErrors(t), e0, upstreamErrors)
}
