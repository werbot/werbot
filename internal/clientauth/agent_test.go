package clientauth

import (
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/platform/sshca"
)

func TestExchangeAgentRequest(t *testing.T) {
	as := newAgentAS(t)
	p := as.profile(t)

	a, err := p.ExchangeAgent(t.Context(), "claude", "claude-code", "fix the build")
	require.NoError(t, err)

	require.Len(t, as.forms, 1)
	f := as.forms[0]
	assert.Equal(t, "urn:ietf:params:oauth:grant-type:token-exchange", f.Get("grant_type"))
	assert.Equal(t, "sponsor-token", f.Get("subject_token"))
	assert.Equal(t, "urn:ietf:params:oauth:token-type:access_token", f.Get("subject_token_type"))
	assert.Equal(t, "urn:ietf:params:oauth:token-type:access_token", f.Get("requested_token_type"))
	assert.Equal(t, "agent:claude", f.Get("scope"))
	assert.Equal(t, "werbot-api", f.Get("audience"))
	assert.Equal(t, "claude-code", f.Get("werbot_client"))
	assert.Equal(t, "fix the build", f.Get("werbot_reason"))
	assert.Equal(t, "werbot-cli", f.Get("client_id"))
	assert.Empty(t, f.Get("refresh_token"))

	assert.Equal(t, as.agentID, a.AgentID)
	assert.Equal(t, as.runID, a.RunID)
	assert.Equal(t, "claude", a.Agent)
	assert.Equal(t, as.srv.URL, a.Server)
	assert.Equal(t, "alice", a.Login)
	assert.WithinDuration(t, time.Now().Add(time.Hour), a.Expiry, time.Minute)
}

func TestExchangeAgentBoundProfile(t *testing.T) {
	as := newAgentAS(t)
	p := as.profile(t)
	p.Agent = "ops"

	_, err := p.ExchangeAgent(t.Context(), "other", "c", "r")
	require.Error(t, err)
	assert.Empty(t, as.forms, "a profile bound to another agent makes no request")

	_, err = p.ExchangeAgent(t.Context(), "ops", "c", "r")
	require.NoError(t, err)
	assert.Equal(t, "agent:ops", as.forms[0].Get("scope"))

	home := t.TempDir()
	require.NoError(t, p.Save(home))
	got, err := Current(home, func(string) string { return "" })
	require.NoError(t, err)
	assert.Equal(t, "ops", got.Agent)
}

func TestExchangeAgentErrors(t *testing.T) {
	as := newAgentAS(t)
	p := as.profile(t)

	as.tokenStatus, as.oauthCode, as.oauthDescription = 400, "invalid_grant", "agent is frozen or locked"
	_, err := p.ExchangeAgent(t.Context(), "claude", "c", "r")
	require.ErrorContains(t, err, "invalid_grant")
	assert.ErrorContains(t, err, "agent is frozen or locked")

	as.oauthCode = ""
	as.tokenStatus = 503
	_, err = p.ExchangeAgent(t.Context(), "claude", "c", "r")
	assert.ErrorContains(t, err, "503")
}

func TestAgentSignerRenews(t *testing.T) {
	as := newAgentAS(t)
	as.certTTLs = []time.Duration{5 * time.Minute, 50 * time.Minute}
	a, err := as.profile(t).ExchangeAgent(t.Context(), "claude", "c", "r")
	require.NoError(t, err)

	s, err := a.Signer(t.Context())
	require.NoError(t, err)
	first := signerCert(t, s)
	assert.Equal(t, as.runID.String(), first.Extensions[sshca.ExtRun])
	assert.Equal(t, as.agentID.String(), first.Extensions[sshca.ExtAgent])
	assert.Equal(t, 1, as.certs)

	s, err = a.Signer(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 2, as.certs, "less than RenewBefore left: a new certificate")
	second := signerCert(t, s)
	assert.NotEqual(t, first.Serial, second.Serial)

	s, err = a.Signer(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 2, as.certs, "enough validity left: the same certificate")
	assert.Equal(t, second.Serial, signerCert(t, s).Serial)

	assert.Equal(t, []string{as.jwt()}, uniq(as.bearers), "certificates are asked with the run token")
}

func TestAgentProfileNotPersisted(t *testing.T) {
	as := newAgentAS(t)
	p := as.profile(t)
	a, err := p.ExchangeAgent(t.Context(), "claude", "c", "r")
	require.NoError(t, err)
	_, err = a.Signer(t.Context())
	require.NoError(t, err)

	var found []string
	_ = filepath.WalkDir(p.home, func(path string, _ fs.DirEntry, _ error) error {
		found = append(found, path)
		return nil
	})
	assert.Equal(t, []string{p.home}, found, "nothing is written to the profile home")

	dir := t.TempDir()
	require.NoError(t, a.WriteKey(dir))
	assertMode(t, filepath.Join(dir, "id_ed25519"), 0o600)
	assertMode(t, filepath.Join(dir, "id_ed25519-cert.pub"), 0o600)
	key, err := ssh.ParsePrivateKey(readFile(t, filepath.Join(dir, "id_ed25519")))
	require.NoError(t, err)
	cert, err := parseCert(readFile(t, filepath.Join(dir, "id_ed25519-cert.pub")), key.PublicKey())
	require.NoError(t, err)
	assert.Equal(t, as.runID.String(), cert.Extensions[sshca.ExtRun])
}

func TestAgentEnd(t *testing.T) {
	as := newAgentAS(t)
	a, err := as.profile(t).ExchangeAgent(t.Context(), "claude", "c", "r")
	require.NoError(t, err)
	require.NoError(t, a.End(t.Context()))
	assert.Equal(t, []string{as.runID.String()}, as.ended)
	assert.Equal(t, []string{as.jwt()}, uniq(as.bearers))
}

func uniq(in []string) []string {
	var out []string
	for _, s := range in {
		if len(out) == 0 || out[len(out)-1] != s {
			out = append(out, s)
		}
	}
	return out
}
