package app

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/anypb"

	sshv1 "github.com/werbot/werbot/gen/werbot/connector/ssh/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/identity/oauth"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// agentRig is a served werbot on pool with its owner: a browser session
// (web) and a CLI access token from the device flow (token).
type agentRig struct {
	base  string
	web   *http.Client
	token string
	pool  *pgxpool.Pool
}

// startAgentRig serves werbot on a fresh database and signs the owner in.
func startAgentRig(t *testing.T) *agentRig {
	t.Helper()
	pool, _ := dbtest.New(t)
	// development: cookie without Secure, which the jar sends over plain http.
	cfg := config.Config{HTTPAddr: freeAddr(t), Env: "development", WebEmbedded: false}
	startServe(t, cfg, pool, nil, nil, slog.New(slog.DiscardHandler))
	r := &agentRig{base: "http://" + cfg.HTTPAddr, pool: pool}
	r.web, _ = jarClient(t, http.DefaultTransport)
	ctx := context.Background()
	_, err := werbotv1connect.NewAuthServiceClient(r.web, r.base).Setup(ctx, connect.NewRequest(ownerSetup))
	require.NoError(t, err)

	var dr struct {
		DeviceCode string `json:"device_code"`
		UserCode   string `json:"user_code"`
	}
	require.Equal(t, http.StatusOK, r.post(t, "/oauth/device_authorization", url.Values{"client_id": {oauth.ClientCLI},
		"scope": {"openid offline_access"}}, false, &dr))
	_, err = werbotv1connect.NewOAuthConsentServiceClient(r.web, r.base).ApproveDeviceCode(ctx,
		connect.NewRequest(&werbotv1.ApproveDeviceCodeRequest{UserCode: dr.UserCode}))
	require.NoError(t, err)
	var tr struct {
		AccessToken string `json:"access_token"`
	}
	require.Equal(t, http.StatusOK, r.post(t, "/oauth/token", url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"},
		"client_id": {oauth.ClientCLI}, "device_code": {dr.DeviceCode}}, false, &tr))
	r.token = tr.AccessToken
	return r
}

// post sends form to path (with the CLI client's Basic auth when basic) and
// decodes the JSON answer into out; it returns the status code.
func (r *agentRig) post(t *testing.T, path string, form url.Values, basic bool, out any) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, r.base+path, strings.NewReader(form.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if basic {
		req.SetBasicAuth(oauth.ClientCLI, "")
	}
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	require.NoError(t, json.NewDecoder(res.Body).Decode(out))
	return res.StatusCode
}

// exchange trades the owner's CLI token for a run token of agent name; it
// returns the status code and the token or the OAuth error.
func (r *agentRig) exchange(t *testing.T, name string) (code int, token, oauthErr string) {
	t.Helper()
	var out struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	code = r.post(t, "/oauth/token", url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:token-exchange"},
		"subject_token": {r.token}, "subject_token_type": {"urn:ietf:params:oauth:token-type:access_token"},
		"scope": {"agent:" + name}, "audience": {oauth.Audience}, "werbot_client": {"claude-code"}}, true, &out)
	return code, out.AccessToken, out.Error
}

// runToken is a run token of agent name; it fails the test otherwise.
func (r *agentRig) runToken(t *testing.T, name string) string {
	t.Helper()
	code, token, oauthErr := r.exchange(t, name)
	require.Equal(t, http.StatusOK, code, oauthErr)
	return token
}

// issueCert asks for a user certificate with token.
func (r *agentRig) issueCert(t *testing.T, token string) error {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	sshPub, err := gossh.NewPublicKey(pub)
	require.NoError(t, err)
	_, err = werbotv1connect.NewCertServiceClient(http.DefaultClient, r.base).IssueUserCert(context.Background(),
		withBearer(&werbotv1.IssueUserCertRequest{PublicKey: string(gossh.MarshalAuthorizedKey(sshPub))}, token))
	return err
}

// agents is the AgentService client of the owner's browser session.
func (r *agentRig) agents() werbotv1connect.AgentServiceClient {
	return werbotv1connect.NewAgentServiceClient(r.web, r.base)
}

// sshConfig is the config of an ssh resource behind a gateway.
func sshConfig(t *testing.T) *anypb.Any {
	t.Helper()
	a, err := anypb.New(&sshv1.Config{Host: "db.internal", Login: "root"})
	require.NoError(t, err)
	return a
}
