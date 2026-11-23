package clientauth

import (
	"cmp"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/sshca"
)

const (
	grantExchange  = "urn:ietf:params:oauth:grant-type:token-exchange"
	tokenTypeAT    = "urn:ietf:params:oauth:token-type:access_token" //nolint:gosec // RFC 8693 type URN, no credential
	agentAudience  = "werbot-api"
	agentActPrefix = "agent:"
)

// AgentProfile is one agent run: its token and ephemeral SSH key live in
// memory only and are never written to the profile (WriteKey is the one
// explicit exception, for werbot agent run).
type AgentProfile struct {
	AgentID, RunID       uuid.UUID
	Agent, Server, Login string
	Expiry               time.Time

	token string
	key   ssh.Signer
	priv  ed25519.PrivateKey

	mu   sync.Mutex
	cert *ssh.Certificate
}

// ExchangeAgent trades the profile token for a run token of agent (RFC 8693).
// A profile bound to an agent exchanges only for that agent, before any request.
func (p *Profile) ExchangeAgent(ctx context.Context, agent, client, reason string) (*AgentProfile, error) {
	agent = cmp.Or(agent, p.Agent)
	if p.Agent != "" && agent != p.Agent {
		return nil, fmt.Errorf("this profile is bound to agent %q", p.Agent)
	}
	subject, err := p.Token(ctx)
	if err != nil {
		return nil, err
	}
	tok, err := tokenRequest(ctx, p.Server, url.Values{
		"grant_type":           {grantExchange},
		"subject_token":        {subject},
		"subject_token_type":   {tokenTypeAT},
		"requested_token_type": {tokenTypeAT},
		"scope":                {agentActPrefix + agent},
		"audience":             {agentAudience},
		"werbot_client":        {client},
		"werbot_reason":        {reason},
	})
	if err != nil {
		return nil, fmt.Errorf("agent token exchange: %w", err)
	}
	agentID, runID, err := runClaims(tok.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("agent token exchange: %w", err)
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	key, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return nil, err
	}
	return &AgentProfile{
		AgentID: agentID, RunID: runID, Agent: agent, Server: p.Server, Login: p.Login,
		Expiry: time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second).UTC(),
		token:  tok.AccessToken, key: key, priv: priv,
	}, nil
}

// runClaims reads the agent (act.sub) and the run (werbot_run) of a token
// from our own server over TLS; the signature is not checked here.
func runClaims(token string) (agent, run uuid.UUID, err error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return agent, run, errors.New("token is not a JWT")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return agent, run, err
	}
	var c struct {
		Act struct {
			Sub string `json:"sub"`
		} `json:"act"`
		Run string `json:"werbot_run"`
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return agent, run, err
	}
	sub, ok := strings.CutPrefix(c.Act.Sub, agentActPrefix)
	if !ok {
		return agent, run, errors.New("token has no agent")
	}
	err = errors.Join(parseUUID(sub, &agent), parseUUID(c.Run, &run))
	return agent, run, err
}

func parseUUID(s string, dst *uuid.UUID) error {
	id, err := uuid.Parse(s)
	if err != nil || id == uuid.Nil {
		return errors.New("token has an invalid agent or run id")
	}
	*dst = id
	return nil
}

// HTTPClient sends the run token as Bearer.
func (a *AgentProfile) HTTPClient() *http.Client {
	return &http.Client{Transport: runBearer{a.token}}
}

type runBearer struct{ token string }

func (b runBearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

// Signer returns the run key with a certificate that has at least
// sshca.RenewBefore left, asking for a new one while the run token lives.
func (a *AgentProfile) Signer(ctx context.Context) (ssh.Signer, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cert == nil || time.Until(validBefore(a.cert)) < sshca.RenewBefore {
		if time.Now().After(a.Expiry) {
			return nil, errors.New("the agent run token has expired")
		}
		cert, err := issueCert(ctx, a.HTTPClient(), a.Server, a.key.PublicKey())
		if err != nil {
			return nil, err
		}
		a.cert = cert
	}
	return ssh.NewCertSigner(a.cert, a.key)
}

// WriteKey saves the run key and the current certificate (0600) into dir for
// werbot agent run; call Signer first.
func (a *AgentProfile) WriteKey(dir string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cert == nil {
		return errors.New("no certificate yet")
	}
	block, err := ssh.MarshalPrivateKey(a.priv, "")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := WriteFile(filepath.Join(dir, "id_ed25519"), pem.EncodeToMemory(block)); err != nil {
		return err
	}
	return WriteFile(filepath.Join(dir, "id_ed25519-cert.pub"), ssh.MarshalAuthorizedKey(a.cert))
}

// End finishes the run on the server.
func (a *AgentProfile) End(ctx context.Context) error {
	_, err := werbotv1connect.NewAgentServiceClient(a.HTTPClient(), a.Server).EndRun(ctx,
		connect.NewRequest(&werbotv1.EndRunRequest{Id: a.RunID.String()}))
	return err
}
