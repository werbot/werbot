package gateway

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/netip"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/werbot/werbot/internal/connector"
)

func init() {
	// "echo" echoes a terminal and reports its window changes; "failing" fails at once
	term := []connector.Kind{connector.KindTerminal}
	for _, n := range []string{"echo", "failing"} {
		connector.Register(connector.Type{Name: n, NewConfig: func() proto.Message { return &emptypb.Empty{} },
			Validate: func(proto.Message) error { return nil }, Actions: []string{n + ".connect", n + ".shell"}, Kinds: term,
			Recorded: term, Endpoint: func(proto.Message) connector.Endpoint { return connector.Endpoint{AnyTarget: true, NoHealth: true} }})
	}
	RegisterConnector(funcConn{typ: "echo", kinds: []Kind{KindTerminal}, serve: echoTerminal})
	RegisterConnector(funcConn{typ: "failing", kinds: []Kind{KindTerminal}, serve: func(context.Context, *Session) error { return ErrLoginFailed }})
}

// echoTerminal writes the initial window, then echoes the input and reports every
// window change until the session ends, recording output and windows as a
// connector does.
func echoTerminal(ctx context.Context, s *Session) error {
	t := s.Terminal
	out := io.MultiWriter(t, recordedOutput{s.Recording})
	_, _ = fmt.Fprintf(out, "win=%dx%d\n", t.Window.Cols, t.Window.Rows)
	go func() { _, _ = io.Copy(out, t) }() // ends when the input does
	resize := t.Resize
	for {
		select {
		case w, ok := <-resize:
			if !ok {
				resize = nil
				continue
			}
			s.Recording.Resize(w.Cols, w.Rows)
			_, _ = fmt.Fprintf(out, "resize=%dx%d\n", w.Cols, w.Rows)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// recordedOutput writes to the recording's output.
type recordedOutput struct{ RecordingSink }

func (r recordedOutput) Write(p []byte) (int, error) {
	r.Output(p)
	return len(p), nil
}

// frame is what a fakeStream's Read returns next.
type frame struct {
	data       []byte
	cols, rows int
	err        error
}

// fakeStream is a browser stream: Read returns the frames sent to in until
// its context ends; Close keeps the first reason.
type fakeStream struct {
	ctx    context.Context
	in     chan frame
	closed chan struct{}

	mu     sync.Mutex
	out    bytes.Buffer
	sizes  []Window
	reason string
}

func (s *fakeStream) Read() ([]byte, int, int, error) {
	select {
	case f := <-s.in:
		return f.data, f.cols, f.rows, f.err
	case <-s.ctx.Done():
		return nil, 0, 0, s.ctx.Err()
	}
}

func (s *fakeStream) Write(p []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.out.Write(p)
	return nil
}

func (s *fakeStream) Resize(cols, rows int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sizes = append(s.sizes, Window{cols, rows})
	return nil
}

func (s *fakeStream) Close(reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reason == "" {
		s.reason = reason
		close(s.closed)
	}
	return nil
}

// output is what the stream got so far.
func (s *fakeStream) output() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.out.String()
}

// windows are the window sizes the stream got so far.
func (s *fakeStream) windows() []Window {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Window(nil), s.sizes...)
}

// closedWith is the close reason of s, within two seconds.
func (s *fakeStream) closedWith(t *testing.T) string {
	t.Helper()
	select {
	case <-s.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("the stream was not closed")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reason
}

// fakeWebControl resolves the tickets it holds; any other is NotFound.
type fakeWebControl struct {
	mu      sync.Mutex
	user    User
	tickets map[uuid.UUID]WebTicket
	auths   int
	opens   int
	opened  chan *fakeStream
}

func (f *fakeWebControl) AuthenticateWebTicket(_ context.Context, ticket uuid.UUID) (User, WebTicket, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.auths++
	wt, ok := f.tickets[ticket]
	if !ok {
		return User{}, WebTicket{}, connect.NewError(connect.CodeNotFound, nil)
	}
	return f.user, wt, nil
}

func (f *fakeWebControl) OpenStream(ctx context.Context, _ uuid.UUID) (relayStream, error) {
	s := &fakeStream{ctx: ctx, in: make(chan frame, 16), closed: make(chan struct{})}
	f.mu.Lock()
	f.opens++
	f.mu.Unlock()
	f.opened <- s
	return s, nil
}

// calls are the AuthenticateWebTicket and OpenStream calls so far.
func (f *fakeWebControl) calls() (auths, opens int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.auths, f.opens
}

// webEnv is a web front serving a pipeline env on a fake web control.
type webEnv struct {
	*pipeEnv
	ctl *fakeWebControl
	wf  *webFront
}

// newWebEnv is a web front on newPipeEnv with the resources "echo" and "fail" added.
func newWebEnv(t *testing.T, maxSessions int) *webEnv {
	t.Helper()
	e := newPipeEnv(t, maxSessions)
	e.m["recho"], e.m["rfail"] = uuid.New(), uuid.New()
	e.f.setPolicy(compileTest(t, e.m, pipePolicies, nil,
		res{"rfake", "fake", "fake"}, res{"rblocker", "blocker", "blocker"}, res{"rpanicky", "panicky", "panicky"},
		res{"rstream", "streamonly", "streamonly"}, res{"rdenied", "denied", "fake"}, res{"recho", "echo", "echo"},
		res{"rfail", "fail", "failing"}))
	return serveWeb(t, e)
}

// serveWeb runs a web front on e until the test ends.
func serveWeb(t *testing.T, e *pipeEnv) *webEnv {
	t.Helper()
	ctl := &fakeWebControl{user: e.f.user, tickets: map[uuid.UUID]WebTicket{}, opened: make(chan *fakeStream, 64)}
	w := &webEnv{pipeEnv: e, ctl: ctl, wf: newWebFront(ctl, e.p.log)}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); _ = w.wf.Serve(ctx, nil, e.p) }()
	t.Cleanup(func() { cancel(); <-done })
	return w
}

var browserIP = netip.MustParseAddr("203.0.113.7")

// terminalTicket is a web ticket for resource from the browser at browserIP.
func terminalTicket(resource string) WebTicket {
	return WebTicket{Mode: "web", Resource: resource, Window: Window{120, 40}, ClientIP: browserIP}
}

// add holds wt under a new ticket id.
func (w *webEnv) add(wt WebTicket) uuid.UUID {
	id := uuid.New()
	w.ctl.mu.Lock()
	defer w.ctl.mu.Unlock()
	w.ctl.tickets[id] = wt
	return id
}

// stream enqueues ticket and returns the stream opened for it, within two seconds.
func (w *webEnv) stream(t *testing.T, ticket uuid.UUID) *fakeStream {
	t.Helper()
	w.wf.enqueue(ticket)
	select {
	case s := <-w.ctl.opened:
		return s
	case <-time.After(2 * time.Second):
		t.Fatal("no stream opened")
		return nil
	}
}

// open opens a stream for a new ticket wt.
func (w *webEnv) open(t *testing.T, wt WebTicket) *fakeStream {
	t.Helper()
	return w.stream(t, w.add(wt))
}

// session is the one session the pipeline runs, within two seconds.
func (w *webEnv) session(t *testing.T) uuid.UUID {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		if ids := w.p.active.ids(); len(ids) == 1 {
			return ids[0]
		}
		select {
		case <-deadline:
			t.Fatal("no running session")
		case <-time.After(5 * time.Millisecond):
		}
	}
}
