package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/clientauth"
	"github.com/werbot/werbot/internal/platform/sshca"
)

// agentFake stands in for the exchange endpoint, CertService, AgentService
// and ResourceService; the harness profile points at it.
type agentFake struct {
	werbotv1connect.UnimplementedCertServiceHandler
	werbotv1connect.UnimplementedAgentServiceHandler
	werbotv1connect.UnimplementedResourceServiceHandler
	profileFake

	h              *harness
	agentID, runID uuid.UUID
	agents         []*werbotv1.Agent
	certLeft       time.Duration // validity left on every issued certificate
	hits           atomic.Int32  // every HTTP request

	mu     sync.Mutex
	forms  []url.Values
	issued int
	ended  []string
}

// useAgentServer points the harness profile at a new agentFake.
func useAgentServer(t *testing.T, h *harness) *agentFake {
	t.Helper()
	f := &agentFake{h: h, agentID: uuid.New(), runID: uuid.New(), certLeft: sshca.UserCertTTL}
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/token", f.token)
	mux.Handle(werbotv1connect.NewCertServiceHandler(f))
	mux.Handle(werbotv1connect.NewAgentServiceHandler(f))
	mux.Handle(werbotv1connect.NewResourceServiceHandler(f))
	mux.Handle(werbotv1connect.NewAgentProfileServiceHandler(&f.profileFake))
	mux.HandleFunc("/oauth/revoke", f.revokeToken)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	f.saveProfile(t, srv.URL, "")
	return f
}

// saveProfile writes the profile of alice on server; agent set makes it an agent profile.
func (f *agentFake) saveProfile(t *testing.T, server, agent string) {
	t.Helper()
	p := &clientauth.Profile{Server: server, Login: "alice", UserID: uuid.New(), OrgID: uuid.New(), Agent: agent,
		AccessToken: "sponsor-token", RefreshToken: "sponsor-refresh", AccessExpiry: time.Now().Add(10 * time.Minute)}
	require.NoError(t, p.Save(f.h.home))
}

func (f *agentFake) token(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	f.mu.Lock()
	f.forms = append(f.forms, r.PostForm)
	f.mu.Unlock()
	seg := func(v any) string {
		raw, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	jwt := seg(map[string]string{"alg": "none"}) + "." + seg(map[string]any{
		"act": map[string]string{"sub": "agent:" + f.agentID.String()}, "werbot_run": f.runID.String(),
	}) + ".sig"
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"access_token": jwt, "token_type": "Bearer", "expires_in": 3600})
}

func (f *agentFake) IssueUserCert(_ context.Context, req *connect.Request[werbotv1.IssueUserCertRequest]) (*connect.Response[werbotv1.IssueUserCertResponse], error) {
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(req.Msg.PublicKey))
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	f.mu.Lock()
	f.issued++
	f.mu.Unlock()
	id := f.h.identity
	id.AgentID, id.RunID = &f.agentID, &f.runID
	cert, err := sshca.Sign(f.h.ca, pub, id, time.Now().Add(f.certLeft-sshca.UserCertTTL), sshca.UserCertTTL)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&werbotv1.IssueUserCertResponse{Certificate: string(ssh.MarshalAuthorizedKey(cert))}), nil
}

func (f *agentFake) ListAccessibleResources(context.Context, *connect.Request[werbotv1.ListAccessibleResourcesRequest]) (*connect.Response[werbotv1.ListAccessibleResourcesResponse], error) {
	return connect.NewResponse(&werbotv1.ListAccessibleResourcesResponse{Resources: f.h.resources}), nil
}

func (f *agentFake) ListAgents(context.Context, *connect.Request[werbotv1.ListAgentsRequest]) (*connect.Response[werbotv1.ListAgentsResponse], error) {
	return connect.NewResponse(&werbotv1.ListAgentsResponse{Agents: f.agents}), nil
}

func (f *agentFake) EndRun(_ context.Context, req *connect.Request[werbotv1.EndRunRequest]) (*connect.Response[werbotv1.EndRunResponse], error) {
	f.mu.Lock()
	f.ended = append(f.ended, req.Msg.Id)
	f.mu.Unlock()
	return connect.NewResponse(&werbotv1.EndRunResponse{}), nil
}

func (f *agentFake) endedRuns() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.ended...)
}

func (f *agentFake) issuedCerts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.issued
}

func (f *agentFake) exchangeForms() []url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]url.Values(nil), f.forms...)
}

// runCtx is harness.run under ctx.
func (h *harness) runCtx(ctx context.Context, args ...string) error {
	root := &cobra.Command{Use: "werbot", SilenceUsage: true, SilenceErrors: true}
	root.SetOut(h.out)
	root.SetErr(h.errOut)
	root.AddCommand(Commands(h.env)...)
	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}
