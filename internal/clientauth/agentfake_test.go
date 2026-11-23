package clientauth

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/sshca"
)

// agentAS stands in for the exchange endpoint, CertService and AgentService.
// ponytail: cli and mcpserver keep their own copies of this fake; extract one
// clientauthtest package when a fourth copy appears.
type agentAS struct {
	werbotv1connect.UnimplementedCertServiceHandler
	werbotv1connect.UnimplementedAgentServiceHandler

	srv              *httptest.Server
	ca               ssh.Signer
	orgID            uuid.UUID
	agentID, runID   uuid.UUID
	tokenStatus      int             // non-zero: /oauth/token answers with it
	oauthCode        string          // with tokenStatus 400: the RFC 6749 error
	oauthDescription string          //
	certTTLs         []time.Duration // remaining validity of each issued certificate, in order

	mu      sync.Mutex
	forms   []url.Values
	bearers []string // Authorization of every Connect call
	certs   int
	ended   []string
}

func newAgentAS(t *testing.T) *agentAS {
	t.Helper()
	_, caKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	ca, err := ssh.NewSignerFromKey(caKey)
	require.NoError(t, err)
	as := &agentAS{ca: ca, orgID: uuid.New(), agentID: uuid.New(), runID: uuid.New()}
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/token", as.token)
	mux.Handle(werbotv1connect.NewCertServiceHandler(as))
	mux.Handle(werbotv1connect.NewAgentServiceHandler(as))
	as.srv = httptest.NewServer(mux)
	t.Cleanup(as.srv.Close)
	return as
}

// profile is a logged-in sponsor profile that is never written to disk.
func (as *agentAS) profile(t *testing.T) *Profile {
	t.Helper()
	return &Profile{Server: as.srv.URL, Login: "alice", AccessToken: "sponsor-token",
		AccessExpiry: time.Now().Add(time.Hour).UTC(), home: t.TempDir()}
}

func (as *agentAS) jwt() string {
	seg := func(v any) string {
		raw, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	return seg(map[string]string{"alg": "none"}) + "." + seg(map[string]any{
		"act": map[string]string{"sub": "agent:" + as.agentID.String()}, "werbot_run": as.runID.String(),
	}) + ".sig"
}

func (as *agentAS) token(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	as.mu.Lock()
	as.forms = append(as.forms, r.PostForm)
	status, code, desc := as.tokenStatus, as.oauthCode, as.oauthDescription
	as.mu.Unlock()
	switch {
	case code != "":
		writeJSON(w, status, map[string]string{"error": code, "error_description": desc})
	case status != 0:
		w.WriteHeader(status)
	default:
		writeJSON(w, http.StatusOK, map[string]any{"access_token": as.jwt(), "token_type": "Bearer", "expires_in": 3600})
	}
}

func (as *agentAS) record(h http.Header) {
	as.mu.Lock()
	as.bearers = append(as.bearers, strings.TrimPrefix(h.Get("Authorization"), "Bearer "))
	as.mu.Unlock()
}

func (as *agentAS) IssueUserCert(_ context.Context, req *connect.Request[werbotv1.IssueUserCertRequest]) (*connect.Response[werbotv1.IssueUserCertResponse], error) {
	as.record(req.Header())
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(req.Msg.PublicKey))
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	as.mu.Lock()
	remaining := sshca.UserCertTTL
	if as.certs < len(as.certTTLs) {
		remaining = as.certTTLs[as.certs]
	}
	as.certs++
	as.mu.Unlock()
	id := sshca.Identity{OrgID: as.orgID, UserID: uuid.New(), CLISessionID: uuid.New(), Login: "alice", AgentID: &as.agentID, RunID: &as.runID}
	cert, err := sshca.Sign(as.ca, pub, id, time.Now().Add(remaining-sshca.UserCertTTL), sshca.UserCertTTL)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&werbotv1.IssueUserCertResponse{Certificate: string(ssh.MarshalAuthorizedKey(cert))}), nil
}

func (as *agentAS) EndRun(_ context.Context, req *connect.Request[werbotv1.EndRunRequest]) (*connect.Response[werbotv1.EndRunResponse], error) {
	as.record(req.Header())
	as.mu.Lock()
	as.ended = append(as.ended, req.Msg.Id)
	as.mu.Unlock()
	return connect.NewResponse(&werbotv1.EndRunResponse{}), nil
}
