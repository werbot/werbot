//go:build unix

package app

import (
	"context"
	"net"
	"net/http"
	"path"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/durationpb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/connectcli"
	"github.com/werbot/werbot/internal/identity/webauthntest"
)

// mfaStack is a stack whose WERBOT_WEB_URL is http://localhost:<http port>:
// passkeys need a domain name (Ruling C5). web is alice's set-up session.
type mfaStack struct {
	*stack
	webURL string
	web    mfaClients
}

// startMFAStack is startStack with WERBOT_WEB_URL http://localhost:<http port>.
func startMFAStack(t *testing.T) *mfaStack {
	t.Helper()
	m := &mfaStack{}
	s, hc := startStackEnv(t, func(httpAddr string) map[string]string {
		_, port, err := net.SplitHostPort(httpAddr)
		require.NoError(t, err)
		m.webURL = "http://localhost:" + port
		return map[string]string{"WERBOT_WEB_URL": m.webURL}
	})
	m.stack, m.web = s, newMFAClients(hc, s.url)
	return m
}

// mfaSSH is an mfaStack with the resource srv, bound to alice, under an MFA
// rule for every user (reuse window 60s, any factor) that the gateway applies.
func mfaSSH(t *testing.T) *mfaStack {
	t.Helper()
	m := startMFAStack(t)
	srv := m.sshReady(t, "srv")
	_, err := m.api.pol.CreateMFARule(context.Background(), connect.NewRequest(&werbotv1.CreateMFARuleRequest{
		Name: "srv-mfa", Subjects: &werbotv1.Subjects{AllUsers: true}, MaxAge: durationpb.New(time.Minute), Strength: "any",
		ResourceSelector: &werbotv1.ResourceSelector{ResourceIds: []string{srv.String()}}}))
	require.NoError(t, err)
	require.Eventually(t, func() bool { // a key alone no longer opens srv
		c, _, err := m.dial(m.gwSSH, "alice+srv", m.key)
		if err == nil {
			_ = c.Close()
		}
		return err != nil
	}, settle, 100*time.Millisecond)
	return m
}

// enrollPasskey registers a passkey of the user of c, signing for webURL.
func enrollPasskey(t *testing.T, c mfaClients, webURL string) *webauthntest.Authenticator {
	t.Helper()
	ctx := context.Background()
	a := webauthntest.New(t, webURL)
	opts, err := c.mfa.BeginWebAuthnRegistration(ctx, connect.NewRequest(&werbotv1.BeginWebAuthnRegistrationRequest{}))
	require.NoError(t, err)
	_, err = c.mfa.FinishWebAuthnRegistration(ctx, connect.NewRequest(&werbotv1.FinishWebAuthnRegistrationRequest{
		Name: "key", Attestation: a.Create(opts.Msg.GetOptions())}))
	require.NoError(t, err)
	return a
}

// login signs email in with the owner password in a new cookie jar; the
// user has no factor.
func login(t *testing.T, base, email string) mfaClients {
	t.Helper()
	hc, _ := jarClient(t, http.DefaultTransport)
	c := newMFAClients(hc, base)
	res, err := c.auth.Login(context.Background(), connect.NewRequest(&werbotv1.LoginRequest{Email: email, Password: ownerPassword}))
	require.NoError(t, err)
	require.False(t, res.Msg.GetMfaRequired())
	return c
}

// kiLogin is an SSH login that answers the gateway's MFA prompt with
// answer(challenge id, prompt). banner holds the auth banners.
type kiLogin struct {
	client  *gossh.Client
	banner  string
	prompts int
	err     error
}

// kiDial logs in to srv as alice with key, then keyboard-interactive.
// The client closes at the test cleanup.
func (m *mfaStack) kiDial(t *testing.T, key gossh.Signer, answer func(id, prompt string) (string, error)) *kiLogin {
	t.Helper()
	l := &kiLogin{}
	ki := func(_, _ string, questions []string, _ []bool) ([]string, error) {
		answers := make([]string, len(questions))
		for i, q := range questions {
			l.prompts++
			u, _, _, ok := connectcli.ParseMFAPrompt(q)
			require.True(t, ok, "prompt %q", q)
			var err error
			if answers[i], err = answer(path.Base(u), q); err != nil {
				return nil, err
			}
		}
		return answers, nil
	}
	m.dials.Add(1)
	l.client, l.err = gossh.Dial("tcp", m.gwSSH, &gossh.ClientConfig{User: "alice+srv",
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(key), gossh.KeyboardInteractive(ki)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), Timeout: 10 * time.Second, //nolint:gosec // test gateway on loopback
		BannerCallback: func(msg string) error { l.banner += msg; return nil }})
	if l.client != nil {
		t.Cleanup(func() { _ = l.client.Close() })
	}
	return l
}

// expire ends challenge id now, as its 60s would.
func (m *mfaStack) expire(t *testing.T, id string) {
	t.Helper()
	_, err := m.pool.Exec(context.Background(), "UPDATE mfa_challenge SET expires_at = now() WHERE id = $1", id)
	require.NoError(t, err)
}
