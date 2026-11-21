package app

import (
	"context"
	"net/http"
	"regexp"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/clientauth"
)

// browser is alice signed in to the web UI with her own cookie jar: the
// browser werbot login sends her to. It approves logins and lists CLI sessions.
type browser struct {
	hc       *http.Client
	consent  werbotv1connect.OAuthConsentServiceClient
	sessions werbotv1connect.CLISessionServiceClient
}

func (s *stack) browser(t *testing.T) browser {
	t.Helper()
	hc, _ := jarClient(t, http.DefaultTransport)
	_, err := werbotv1connect.NewAuthServiceClient(hc, s.url).Login(context.Background(),
		connect.NewRequest(&werbotv1.LoginRequest{Email: "alice@example.com", Password: ownerPassword}))
	require.NoError(t, err)
	return browser{hc: hc, consent: werbotv1connect.NewOAuthConsentServiceClient(hc, s.url),
		sessions: werbotv1connect.NewCLISessionServiceClient(hc, s.url)}
}

// approveLoopback is LoginOptions.OpenBrowser: it opens the authorization
// URL, reads the request id from the redirect to the consent page, approves
// it and follows the callback to the CLI's loopback listener.
func (b browser) approveLoopback(t *testing.T) func(string) error {
	return func(authURL string) error {
		t.Helper()
		noFollow := &http.Client{Jar: b.hc.Jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		res, err := noFollow.Get(authURL) //nolint:noctx // test browser
		require.NoError(t, err)
		_ = res.Body.Close()
		loc, err := res.Location()
		require.NoError(t, err, "authorize answers %d", res.StatusCode)
		require.Equal(t, "/cli/authorize", loc.Path)
		ok, err := b.consent.ApproveAuthRequest(context.Background(),
			connect.NewRequest(&werbotv1.ApproveAuthRequestRequest{Id: loc.Query().Get("id")}))
		require.NoError(t, err)
		res, err = b.hc.Get(ok.Msg.GetRedirectUrl()) //nolint:noctx // test browser; ends at the loopback listener
		require.NoError(t, err)
		return res.Body.Close()
	}
}

var userCode = regexp.MustCompile(`enter code (\S+)`)

// cliLogin runs werbot login as alice against the stack's server, with the
// device flow (approved from the code the CLI prints) or the loopback flow.
func (s *stack) cliLogin(t *testing.T, device bool) (home string, p *clientauth.Profile) {
	t.Helper()
	ctx, b, home := context.Background(), s.browser(t), t.TempDir()
	if !device {
		p, err := clientauth.Login(ctx, home, s.url, clientauth.LoginOptions{OpenBrowser: b.approveLoopback(t)})
		require.NoError(t, err)
		return home, p
	}
	var out syncBuffer
	type result struct {
		p   *clientauth.Profile
		err error
	}
	done := make(chan result, 1)
	go func() {
		p, err := clientauth.Login(t.Context(), home, s.url, clientauth.LoginOptions{Device: true, Out: &out})
		done <- result{p, err}
	}()
	require.Eventually(t, func() bool { return userCode.MatchString(out.String()) }, 10*time.Second, 10*time.Millisecond)
	_, err := b.consent.ApproveDeviceCode(ctx, connect.NewRequest(&werbotv1.ApproveDeviceCodeRequest{
		UserCode: userCode.FindStringSubmatch(out.String())[1]}))
	require.NoError(t, err)
	select {
	case r := <-done: // the first poll ran before the approval: one 5s interval
		require.NoError(t, r.err)
		return home, r.p
	case <-time.After(30 * time.Second):
		t.Fatal("device login did not finish within 30s")
		return "", nil
	}
}

// setSSHAddress points the pool "gw" at the gateway's SSH listener, as an admin does.
func (s *stack) setSSHAddress(t *testing.T) {
	t.Helper()
	_, err := s.api.gws.UpdateGateway(context.Background(), connect.NewRequest(&werbotv1.UpdateGatewayRequest{
		Id: s.gatewayID.String(), SshAddress: s.gwSSH}))
	require.NoError(t, err)
}

// certExec runs cmd on resource srv as alice with the profile's key and
// certificate; a connection failure is returned as is.
func (s *stack) certExec(t *testing.T, home string, p *clientauth.Profile, cmd string) (string, error) {
	t.Helper()
	signer, err := p.Signer(context.Background(), home)
	require.NoError(t, err)
	c, _, err := s.dial(s.gwSSH, "alice+srv", signer)
	if err != nil {
		return "", err
	}
	defer c.Close() //nolint:errcheck // test client
	return run(c, cmd)
}

// run runs cmd in a new session of c and returns its stdout.
func run(c *gossh.Client, cmd string) (string, error) {
	sess, err := c.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close() //nolint:errcheck // closed by the server after exit
	b, err := sess.Output(cmd)
	return string(b), err
}
