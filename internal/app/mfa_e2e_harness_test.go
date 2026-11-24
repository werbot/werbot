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
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/connectcli"
	"github.com/werbot/werbot/internal/gateway/sshtest"
	"github.com/werbot/werbot/internal/identity/webauthntest"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// mfaClients are the MFA clients of one browser session.
type mfaClients struct {
	hc    *http.Client
	auth  werbotv1connect.AuthServiceClient
	mfa   werbotv1connect.MFAServiceClient
	chals werbotv1connect.MFAChallengeServiceClient
}

func newMFAClients(hc *http.Client, base string) mfaClients {
	return mfaClients{hc: hc, auth: werbotv1connect.NewAuthServiceClient(hc, base),
		mfa: werbotv1connect.NewMFAServiceClient(hc, base), chals: werbotv1connect.NewMFAChallengeServiceClient(hc, base)}
}

// mfaStack is a stack whose WERBOT_WEB_URL is http://localhost:<http port>:
// passkeys need a domain name (Ruling C5). web is alice's set-up session.
type mfaStack struct {
	*stack
	webURL string
	web    mfaClients
}

// startMFAStack is startStack with webURL as WERBOT_WEB_URL.
func startMFAStack(t *testing.T) *mfaStack {
	t.Helper()
	pool, dbURL := dbtest.New(t)
	httpAddr := freeAddr(t)
	_, port, err := net.SplitHostPort(httpAddr)
	require.NoError(t, err)
	m := &mfaStack{webURL: "http://localhost:" + port}
	r := startRun(t, []string{"server"}, env(map[string]string{
		"WERBOT_DATABASE_URL": dbURL, "WERBOT_HTTP_ADDR": httpAddr, "WERBOT_PUBLIC_URL": "http://" + httpAddr,
		"WERBOT_WEB_URL": m.webURL, "WERBOT_SECRET_KEY": testSecretKey, "WERBOT_GATEWAY_ADDR": freeAddr(t),
		"WERBOT_DATA_DIR": t.TempDir(), "WERBOT_ENV": "development",
	})).healthy(t)
	s := &stack{pool: pool, url: r.api, target: sshtest.StartTarget(t, "target-secret", nil)}
	m.stack = s
	ctx := context.Background()

	hc, _ := jarClient(t, http.DefaultTransport)
	m.web = newMFAClients(hc, s.url)
	setup := proto.CloneOf(ownerSetup)
	setup.Login, setup.Email = "alice", "alice@example.com"
	_, err = m.web.auth.Setup(ctx, connect.NewRequest(setup))
	require.NoError(t, err)
	me, err := m.web.auth.GetSession(ctx, connect.NewRequest(&werbotv1.GetSessionRequest{}))
	require.NoError(t, err)
	s.alice = me.Msg.GetUser().GetId()
	s.api = apiClients{
		keys: werbotv1connect.NewSSHKeyServiceClient(hc, s.url), res: werbotv1connect.NewResourceServiceClient(hc, s.url),
		acc: werbotv1connect.NewAccessServiceClient(hc, s.url), pol: werbotv1connect.NewPolicyServiceClient(hc, s.url),
		gws: werbotv1connect.NewGatewayServiceClient(hc, s.url),
	}
	s.key = m.addKey(t)
	gw, err := s.api.gws.CreateGateway(ctx, connect.NewRequest(&werbotv1.CreateGatewayRequest{Name: "gw"}))
	require.NoError(t, err)
	s.gatewayID = uuid.MustParse(gw.Msg.GetGateway().GetId())
	tok, err := s.api.gws.CreateJoinToken(ctx, connect.NewRequest(&werbotv1.CreateJoinTokenRequest{
		GatewayId: s.gatewayID.String(), MaxUses: 2}))
	require.NoError(t, err)
	s.token = tok.Msg.GetToken()
	s.gwSSH, s.gwData, err = s.startGateway(t, s.token)
	require.NoError(t, err)
	return m
}

// addKey registers a new SSH key of alice.
func (m *mfaStack) addKey(t *testing.T) gossh.Signer {
	t.Helper()
	key := newSigner(t)
	_, err := m.api.keys.AddSSHKey(context.Background(), connect.NewRequest(&werbotv1.AddSSHKeyRequest{
		PublicKey: string(gossh.MarshalAuthorizedKey(key.PublicKey()))}))
	require.NoError(t, err)
	return key
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

// enrollTOTP gives the user of c a TOTP factor, confirmed with the code of
// the previous step, and returns its secret.
func enrollTOTP(t *testing.T, c mfaClients) string {
	t.Helper()
	ctx := context.Background()
	enr, err := c.mfa.EnrollTOTP(ctx, connect.NewRequest(&werbotv1.EnrollTOTPRequest{}))
	require.NoError(t, err)
	_, err = c.mfa.ConfirmTOTP(ctx, connect.NewRequest(&werbotv1.ConfirmTOTPRequest{Code: totpAt(t, enr.Msg.GetSecret(), -30*time.Second)}))
	require.NoError(t, err)
	return enr.Msg.GetSecret()
}

// totpAt is the TOTP code of secret at now+d. Codes taken at increasing d
// belong to increasing steps, so each is accepted once.
func totpAt(t *testing.T, secret string, d time.Duration) string {
	t.Helper()
	code, err := totp.GenerateCode(secret, time.Now().Add(d))
	require.NoError(t, err)
	return code
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
