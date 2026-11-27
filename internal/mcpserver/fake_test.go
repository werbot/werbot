package mcpserver

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	gliderssh "github.com/gliderlabs/ssh"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/clientauth"
	"github.com/werbot/werbot/internal/platform/sshca"
)

type finished struct{ id, status string }

// fakeAPI stands in for the exchange endpoint and the Connect services an
// agent run calls.
type fakeAPI struct {
	werbotv1connect.UnimplementedCertServiceHandler
	werbotv1connect.UnimplementedResourceServiceHandler
	werbotv1connect.UnimplementedAgentServiceHandler
	werbotv1connect.UnimplementedAccessRequestServiceHandler

	srv            *httptest.Server
	ca             gossh.Signer
	agentID, runID uuid.UUID

	mu          sync.Mutex
	exchangeErr string         // non-empty: the exchange answers invalid_grant with it
	recordErr   *connect.Error // non-nil: RecordToolCall answers it
	resources   []*werbotv1.AccessibleResource
	run         *werbotv1.AgentRun
	requests    []*werbotv1.AccessRequest
	exchanges   []url.Values
	lists       int
	toolCalls   []*werbotv1.RecordToolCallRequest
	finishes    []finished
	created     []*werbotv1.CreateRequestRequest
	ended       []string
}

func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()
	_, caKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	ca, err := gossh.NewSignerFromKey(caKey)
	require.NoError(t, err)
	a := &fakeAPI{ca: ca, agentID: uuid.New(), runID: uuid.New()}
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/token", a.token)
	mux.Handle(werbotv1connect.NewCertServiceHandler(a))
	mux.Handle(werbotv1connect.NewResourceServiceHandler(a))
	mux.Handle(werbotv1connect.NewAgentServiceHandler(a))
	mux.Handle(werbotv1connect.NewAccessRequestServiceHandler(a))
	a.srv = httptest.NewServer(mux)
	t.Cleanup(a.srv.Close)
	return a
}

// profile is a logged-in sponsor profile that is never written to disk.
func (a *fakeAPI) profile() *clientauth.Profile {
	return &clientauth.Profile{Server: a.srv.URL, Login: "alice", AccessToken: "sponsor-token",
		AccessExpiry: time.Now().Add(time.Hour).UTC()}
}

func (a *fakeAPI) jwt() string {
	seg := func(v any) string {
		raw, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	return seg(map[string]string{"alg": "none"}) + "." + seg(map[string]any{
		"act": map[string]string{"sub": "agent:" + a.agentID.String()}, "werbot_run": a.runID.String(),
	}) + ".sig"
}

func (a *fakeAPI) token(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	a.mu.Lock()
	a.exchanges = append(a.exchanges, r.PostForm)
	desc := a.exchangeErr
	a.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if desc != "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant", "error_description": desc})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"access_token": a.jwt(), "token_type": "Bearer", "expires_in": 3600})
}

// state reads the recorded fields under the lock.
func state[T any](a *fakeAPI, f func() T) T {
	a.mu.Lock()
	defer a.mu.Unlock()
	return f()
}

func (a *fakeAPI) forms() []url.Values {
	return state(a, func() []url.Values { return slices.Clone(a.exchanges) })
}

func (a *fakeAPI) endedRuns() []string {
	return state(a, func() []string { return slices.Clone(a.ended) })
}

func (a *fakeAPI) set(f func()) {
	a.mu.Lock()
	defer a.mu.Unlock()
	f()
}

func (a *fakeAPI) IssueUserCert(_ context.Context, req *connect.Request[werbotv1.IssueUserCertRequest]) (*connect.Response[werbotv1.IssueUserCertResponse], error) {
	pub, _, _, _, err := gossh.ParseAuthorizedKey([]byte(req.Msg.PublicKey))
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	id := sshca.Identity{OrgID: uuid.New(), UserID: uuid.New(), CLISessionID: uuid.New(), Login: "alice", AgentID: &a.agentID, RunID: &a.runID}
	cert, err := sshca.Sign(a.ca, pub, id, time.Now(), sshca.UserCertTTL)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&werbotv1.IssueUserCertResponse{Certificate: string(gossh.MarshalAuthorizedKey(cert))}), nil
}

func (a *fakeAPI) ListAccessibleResources(context.Context, *connect.Request[werbotv1.ListAccessibleResourcesRequest]) (*connect.Response[werbotv1.ListAccessibleResourcesResponse], error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lists++
	return connect.NewResponse(&werbotv1.ListAccessibleResourcesResponse{Resources: a.resources}), nil
}

func (a *fakeAPI) RecordToolCall(_ context.Context, req *connect.Request[werbotv1.RecordToolCallRequest]) (*connect.Response[werbotv1.RecordToolCallResponse], error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.recordErr != nil {
		return nil, a.recordErr
	}
	a.toolCalls = append(a.toolCalls, req.Msg)
	return connect.NewResponse(&werbotv1.RecordToolCallResponse{Id: uuid.NewString()}), nil
}

func (a *fakeAPI) FinishToolCall(_ context.Context, req *connect.Request[werbotv1.FinishToolCallRequest]) (*connect.Response[werbotv1.FinishToolCallResponse], error) {
	a.set(func() { a.finishes = append(a.finishes, finished{req.Msg.Id, req.Msg.Status}) })
	return connect.NewResponse(&werbotv1.FinishToolCallResponse{}), nil
}

func (a *fakeAPI) GetRun(_ context.Context, req *connect.Request[werbotv1.GetRunRequest]) (*connect.Response[werbotv1.GetRunResponse], error) {
	if req.Msg.Id != a.runID.String() {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	return connect.NewResponse(&werbotv1.GetRunResponse{Run: state(a, func() *werbotv1.AgentRun { return a.run })}), nil
}

func (a *fakeAPI) EndRun(_ context.Context, req *connect.Request[werbotv1.EndRunRequest]) (*connect.Response[werbotv1.EndRunResponse], error) {
	a.set(func() { a.ended = append(a.ended, req.Msg.Id) })
	return connect.NewResponse(&werbotv1.EndRunResponse{}), nil
}

func (a *fakeAPI) CreateRequest(_ context.Context, req *connect.Request[werbotv1.CreateRequestRequest]) (*connect.Response[werbotv1.CreateRequestResponse], error) {
	a.set(func() { a.created = append(a.created, req.Msg) })
	return connect.NewResponse(&werbotv1.CreateRequestResponse{Request: &werbotv1.AccessRequest{
		Id: "r1", Status: "pending", Url: "https://werbot.test/requests?id=r1", ResourceId: req.Msg.ResourceId}}), nil
}

func (a *fakeAPI) ListRequests(_ context.Context, req *connect.Request[werbotv1.ListRequestsRequest]) (*connect.Response[werbotv1.ListRequestsResponse], error) {
	var out []*werbotv1.AccessRequest
	for _, r := range state(a, func() []*werbotv1.AccessRequest { return a.requests }) {
		if req.Msg.Status == "" || r.Status == req.Msg.Status {
			out = append(out, r)
		}
	}
	return connect.NewResponse(&werbotv1.ListRequestsResponse{Requests: out}), nil
}

// fakeGateway is an SSH server that accepts only certificates of the fake
// CA with an agent run and runs handle for every session.
type fakeGateway struct {
	addr, hostKey string

	mu       sync.Mutex
	users    []string
	envs     [][]string
	commands []string
	closed   chan struct{} // closed when a session's client goes away
}

func newFakeGateway(t *testing.T, ca gossh.PublicKey, handle func(*fakeGateway, gliderssh.Session)) *fakeGateway {
	t.Helper()
	return startGateway(t, ca, func(g *fakeGateway, srv *gliderssh.Server) {
		srv.Handler = func(s gliderssh.Session) {
			g.mu.Lock()
			g.users, g.envs, g.commands = append(g.users, s.User()), append(g.envs, s.Environ()), append(g.commands, s.RawCommand())
			g.mu.Unlock()
			handle(g, s)
		}
	})
}

// startGateway serves the SSH server that setup completes, accepting only
// certificates of ca with an agent run.
func startGateway(t *testing.T, ca gossh.PublicKey, setup func(*fakeGateway, *gliderssh.Server)) *fakeGateway {
	t.Helper()
	_, hostKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	host, err := gossh.NewSignerFromKey(hostKey)
	require.NoError(t, err)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	g := &fakeGateway{addr: l.Addr().String(), closed: make(chan struct{}, 8),
		hostKey: strings.TrimSpace(string(gossh.MarshalAuthorizedKey(host.PublicKey())))}
	srv := &gliderssh.Server{
		HostSigners: []gliderssh.Signer{host},
		PublicKeyHandler: func(_ gliderssh.Context, key gliderssh.PublicKey) bool {
			cert, ok := key.(*gossh.Certificate)
			return ok && string(cert.SignatureKey.Marshal()) == string(ca.Marshal()) && cert.Extensions[sshca.ExtRun] != ""
		},
	}
	setup(g, srv)
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = srv.Close() })
	return g
}

// resource is the accessible resource "srv" behind g.
func (g *fakeGateway) resource() *werbotv1.AccessibleResource {
	return &werbotv1.AccessibleResource{Id: uuid.NewString(), Name: "srv", Type: "ssh", Kinds: []string{"exec"},
		SshAddress: g.addr, HostKey: g.hostKey, Allowed: true, AllowedClasses: []string{"read"}}
}

func (g *fakeGateway) seen() (users []string, envs [][]string, commands []string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.users, g.envs, g.commands
}

// silent waits for the client to leave and reports it on g.closed.
func (g *fakeGateway) silent(s gliderssh.Session) {
	<-s.Context().Done()
	g.closed <- struct{}{}
}

func runOf(a *fakeAPI) *werbotv1.AgentRun {
	return &werbotv1.AgentRun{Id: a.runID.String(), Status: "running", Actions: 7, MaxActions: 100,
		ExpiresAt: timestamppb.New(time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC))}
}
