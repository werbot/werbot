package gateway

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/platform/metrics"
)

func init() {
	// server-side types: the local PDP requires "<type>.connect" among the type's actions
	all := []connector.Kind{connector.KindTerminal, connector.KindExec, connector.KindStream}
	for _, n := range []string{"fake", "blocker", "panicky", "streamonly", "nope", "spy"} {
		connector.Register(connector.Type{Name: n, NewConfig: func() proto.Message { return &emptypb.Empty{} },
			Validate: func(proto.Message) error { return nil }, Actions: []string{n + ".connect"}, Kinds: all,
			Recorded: all, Endpoint: func(proto.Message) connector.Endpoint { return connector.Endpoint{AnyTarget: true, NoHealth: true} }})
	}
	// gateway connectors; "nope" stays without one
	RegisterConnector(fakeConn{})
	RegisterConnector(funcConn{typ: "blocker", kinds: []Kind{KindTerminal, KindExec, KindStream}, serve: block})
	RegisterConnector(funcConn{typ: "panicky", kinds: []Kind{KindTerminal}, serve: func(context.Context, *Session) error { panic("boom") }})
	RegisterConnector(funcConn{typ: "streamonly", kinds: []Kind{KindStream}, serve: block})
	RegisterConnector(funcConn{typ: "spy", kinds: []Kind{KindTerminal, KindExec, KindStream}, serve: spy})
}

func block(ctx context.Context, _ *Session) error {
	<-ctx.Done()
	return ctx.Err()
}

// spy prints what the front handed to the connector. A terminal running
// "resize" also prints the first Resize, a terminal of the resource "typeahead"
// the first input line; an exec exits with 7.
func spy(ctx context.Context, s *Session) error {
	switch {
	case s.Terminal != nil:
		t := s.Terminal
		_, _ = fmt.Fprintf(t, "spy:%s kind=terminal term=%q cmd=%q win=%dx%d\n", s.Resource.Name, t.Term, t.Command, t.Window.Cols, t.Window.Rows)
		if t.Command == "resize" {
			select {
			case w := <-t.Resize:
				_, _ = fmt.Fprintf(t, "resize=%dx%d\n", w.Cols, w.Rows)
			case <-time.After(2 * time.Second):
				_, _ = fmt.Fprintln(t, "resize=none")
			case <-ctx.Done():
			}
		}
		if s.Resource.Name == "typeahead" {
			line, _ := bufio.NewReader(t).ReadString('\r')
			_, _ = fmt.Fprintf(t, "typed=%q\n", line)
		}
	case s.Exec != nil:
		_, _ = fmt.Fprintf(s.Exec.Stdout, "spy:%s kind=exec cmd=%q\n", s.Resource.Name, s.Exec.Command)
		s.Exec.SetExitStatus(7)
	case s.Stream != nil:
		_, _ = fmt.Fprintf(s.Stream, "spy:%s kind=stream target=%s\n", s.Resource.Name, s.Target)
	}
	return nil
}

// fakeConn serves every kind and writes one line naming the session.
type fakeConn struct{}

func (fakeConn) Type() string  { return "fake" }
func (fakeConn) Kinds() []Kind { return []Kind{KindTerminal, KindExec, KindStream} }
func (fakeConn) Serve(_ context.Context, s *Session) error {
	w := io.Discard
	switch {
	case s.Terminal != nil:
		w = s.Terminal
	case s.Exec != nil:
		w = s.Exec.Stdout
	case s.Stream != nil:
		w = s.Stream
	}
	_, err := fmt.Fprintf(w, "fake:%s user=%s secret=%s kind=%s\n", s.Resource.Name, s.User.Login, s.Secret.Value, s.Kind)
	return err
}

// funcConn is a test connector built from a function.
type funcConn struct {
	typ   string
	kinds []Kind
	serve func(context.Context, *Session) error
}

func (c funcConn) Type() string                                { return c.typ }
func (c funcConn) Kinds() []Kind                               { return c.kinds }
func (c funcConn) Serve(ctx context.Context, s *Session) error { return c.serve(ctx, s) }

type endCall struct {
	id     uuid.UUID
	reason string
}

// fakeControl is the control plane of a pipeline and the uploader of its spool;
// events keeps EndSession and Recording.Close in call order.
type fakeControl struct {
	mu         sync.Mutex
	policy     *Policy
	connected  bool
	user       User
	authErr    error
	byKey      map[string]User  // AuthenticateUser by fingerprint when set; unknown → NotFound
	keyErrs    map[string]error // AuthenticateUser error by fingerprint
	startErr   error
	upErr      error // returned by every uploader call: the server is down
	pinErr     error
	authCalls  []string
	startCalls int
	starts     []string // "<user id> <resource id> <ip>"
	ends       []endCall
	pins       []string
	events     []string
	recs       []string
}

func (f *fakeControl) AuthenticateUser(_ context.Context, fp string, ip netip.Addr) (User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.authCalls = append(f.authCalls, fp+" "+ip.String())
	if f.byKey == nil || f.authErr != nil {
		return f.user, f.authErr
	}
	if err := f.keyErrs[fp]; err != nil {
		return User{}, err
	}
	u, ok := f.byKey[fp]
	if !ok {
		return User{}, connect.NewError(connect.CodeNotFound, nil)
	}
	return u, nil
}

func (f *fakeControl) StartSession(_ context.Context, userID, resourceID uuid.UUID, ip netip.Addr) (uuid.UUID, Secret, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.startCalls++
	f.starts = append(f.starts, fmt.Sprintf("%s %s %s", userID, resourceID, ip))
	if f.startErr != nil {
		return uuid.Nil, Secret{}, f.startErr
	}
	if f.byKey == nil && userID != f.user.ID {
		return uuid.Nil, Secret{}, fmt.Errorf("unexpected user %s", userID)
	}
	return uuid.New(), Secret{Kind: "password", Value: []byte("s3cret-value")}, nil
}

// EndSession records a delivered end.
func (f *fakeControl) EndSession(_ context.Context, id uuid.UUID, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.upErr != nil {
		return f.upErr
	}
	f.ends = append(f.ends, endCall{id, reason})
	f.events = append(f.events, "end:"+reason)
	return nil
}

func (f *fakeControl) PinHostKey(_ context.Context, resourceID uuid.UUID, alg string, key []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pins = append(f.pins, fmt.Sprintf("%s %s %s", resourceID, alg, key))
	return f.pinErr
}

func (f *fakeControl) Policy() *Policy {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.policy
}

func (f *fakeControl) Connected() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.connected
}

func (f *fakeControl) with(fn func(*fakeControl)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f)
}

// endsOf lists the EndSession reasons sent for id.
func (f *fakeControl) endsOf(id uuid.UUID) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, e := range f.ends {
		if e.id == id {
			out = append(out, e.reason)
		}
	}
	return out
}

// spySink notes its Close in the control's event log: "close" when the session
// end was spooled (pending or already delivered) before, "close-before-end" otherwise.
type spySink struct {
	f  *fakeControl
	p  *Pipeline
	id uuid.UUID
}

func (spySink) Output([]byte)   {}
func (spySink) Input([]byte)    {}
func (spySink) Resize(int, int) {}
func (s spySink) Close(context.Context) error {
	ev := "close-before-end"
	if slices.Contains(s.p.spool.PendingEnds(), s.id) || len(s.f.endsOf(s.id)) > 0 { // delivery records before it leaves PendingEnds
		ev = "close"
	}
	s.f.with(func(f *fakeControl) { f.events = append(f.events, ev) })
	return nil
}

// pipeEnv is a pipeline on a fake control plane with the test snapshot.
type pipeEnv struct {
	p   *Pipeline
	f   *fakeControl
	reg *metrics.Registry
	log *syncBuffer
	m   ids
}

const pipePolicies = `
permit (principal == Werbot::User::"u1", action, resource);
forbid (principal, action, resource == Werbot::Resource::"rdenied");`

func newPipeEnv(t *testing.T, maxSessions int) *pipeEnv {
	t.Helper()
	m := newIDs("u1", "rfake", "rblocker", "rpanicky", "rstream", "rnope", "rdenied")
	pol := compileTest(t, m, pipePolicies, nil,
		res{"rfake", "fake", "fake"}, res{"rblocker", "blocker", "blocker"}, res{"rpanicky", "panicky", "panicky"},
		res{"rstream", "streamonly", "streamonly"}, res{"rnope", "nope", "nope"}, res{"rdenied", "denied", "fake"})
	e := &pipeEnv{reg: metrics.NewRegistry(), log: &syncBuffer{}, m: m}
	e.f = &fakeControl{policy: pol, connected: true, user: User{ID: m["u1"], Login: "alice", Country: "NL"}}
	rec := func(id uuid.UUID, cols, rows int) RecordingSink {
		e.f.with(func(f *fakeControl) { f.recs = append(f.recs, fmt.Sprintf("%s %d %d", id, cols, rows)) })
		return spySink{f: e.f, p: e.p, id: id}
	}
	cfg := Config{MaxSessions: maxSessions, Recording: rec, Metrics: e.reg}
	e.p = newPipeline(e.f, cfg, slog.New(slog.NewTextHandler(e.log, nil)))
	e.p.spool = newTestSpool(t, e.f)
	t.Cleanup(e.p.stopNow)
	return e
}

func (e *pipeEnv) req(resource string, kind Kind) OpenRequest {
	return OpenRequest{Front: "ssh", Kind: kind, Identity: Identity{Method: "ssh-key", Value: "SHA256:key"},
		Resource: resource, ClientIP: netip.MustParseAddr("192.0.2.7"), Window: Window{Cols: 120, Rows: 40}}
}

func (e *pipeEnv) open(t *testing.T, resource string, kind Kind) *Session {
	t.Helper()
	s, err := e.p.Open(t.Context(), e.req(resource, kind))
	if err != nil {
		t.Fatalf("open %s: %v", resource, err)
	}
	return s
}

func (e *pipeEnv) metrics(t *testing.T) string {
	t.Helper()
	rec := httptest.NewRecorder()
	e.reg.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	return rec.Body.String()
}

// inHeartbeat reports whether id is among the heartbeat's active session ids.
func (e *pipeEnv) inHeartbeat(id uuid.UUID) bool {
	return slices.Contains(e.p.heartbeat().GetActiveSessionIds(), id.String())
}
