package relay

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/principal"
)

const webURL = "https://ui.example.com"

// wait bounds every blocking step of the WebSocket tests, so a regression
// fails fast instead of hanging until -timeout.
const wait = 10 * time.Second

var (
	member   = principal.Principal{UserID: uuid.New(), OrgID: uuid.New(), Role: principal.RoleMember}
	agentRun = uuid.New()
	agent    = principal.Principal{UserID: uuid.New(), OrgID: uuid.New(), Role: principal.RoleMember, AgentID: &agentRun, RunID: &agentRun}
)

// spyTickets records every ticket request before passing it on.
type spyTickets struct {
	*fakeTickets
	mu    sync.Mutex
	calls []string
}

func (s *spyTickets) record(call string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, call)
}

func (s *spyTickets) list() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.calls)
}

func (s *spyTickets) CreateObserve(ctx context.Context, p principal.Principal, id, target uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	s.record(fmt.Sprintf("observe %s %s", id, target))
	return s.fakeTickets.CreateObserve(ctx, p, id, target)
}

func (s *spyTickets) CreateTerminal(ctx context.Context, p principal.Principal, res string, cols, rows int, target uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	s.record(fmt.Sprintf("terminal %s %dx%d %s", res, cols, rows, target))
	return s.fakeTickets.CreateTerminal(ctx, p, res, cols, rows, target)
}

// idleRelay is a relay without a database that counts as running: tickets
// fail with err (nil: the tests never reach the gateway).
func idleRelay(t *testing.T, err error) (*Relay, *spyTickets) {
	t.Helper()
	spy := &spyTickets{fakeTickets: &fakeTickets{m: map[uuid.UUID]Ticket{}, err: err}}
	r := New(nil, newCA(t), spy, Config{InstanceID: uuid.New(), WebURL: webURL}, slog.New(slog.DiscardHandler))
	r.registered.Store(true)
	return r, spy
}

// wsRelay is one started replica whose gateway bridges every stream on it.
func wsRelay(t *testing.T, tune func(r *Relay)) (*cluster, *spyTickets, *fakeGateway) {
	t.Helper()
	var spy *spyTickets
	c := newCluster(t, 1, func(_ int, r *Relay) {
		r.cfg.WebURL = webURL
		spy = &spyTickets{fakeTickets: r.tickets.(*fakeTickets)}
		r.tickets = spy
		if tune != nil {
			tune(r)
		}
	})
	return c, spy, startGateway(t, c, c.rs[0])
}

// serveWS serves r's WebSocket handler with p as the request's principal
// (nil: unauthenticated).
func serveWS(t *testing.T, r *Relay, p *principal.Principal) *httptest.Server {
	t.Helper()
	h := r.WebSocketHandler()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if p != nil {
			req = req.WithContext(principal.With(req.Context(), *p))
		}
		h.ServeHTTP(w, req)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// dial opens path on srv with origin ("" sends none).
func dial(t *testing.T, srv *httptest.Server, path, origin string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	hdr := http.Header{}
	if origin != "" {
		hdr.Set("Origin", origin)
	}
	return websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+path, &websocket.DialOptions{HTTPHeader: hdr})
}

func mustDial(t *testing.T, srv *httptest.Server, path string) *websocket.Conn {
	t.Helper()
	c, _, err := dial(t, srv, path, webURL) //nolint:bodyclose // websocket.Dial owns the body
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.CloseNow() })
	return c
}

// refused dials path and returns the HTTP status that refused the upgrade.
func refused(t *testing.T, srv *httptest.Server, path, origin string) int {
	t.Helper()
	_, res, err := dial(t, srv, path, origin) //nolint:bodyclose // websocket.Dial owns the body
	require.Error(t, err, path)
	require.NotNil(t, res, path)
	return res.StatusCode
}

func send(t *testing.T, c *websocket.Conn, typ websocket.MessageType, b string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	require.NoError(t, c.Write(ctx, typ, []byte(b)))
}

func recv(t *testing.T, c *websocket.Conn) (websocket.MessageType, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	typ, b, err := c.Read(ctx)
	require.NoError(t, err)
	return typ, string(b)
}

// closed reads c until the server closes it and returns the code and reason.
func closed(t *testing.T, c *websocket.Conn) (websocket.StatusCode, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	for {
		_, _, err := c.Read(ctx)
		if err == nil {
			continue
		}
		var ce websocket.CloseError
		require.True(t, errors.As(err, &ce), "want a close frame, got %v", err)
		return ce.Code, ce.Reason
	}
}

// within runs f and fails the test when it does not return within wait.
func within[T any](t *testing.T, f func() T) T {
	t.Helper()
	ch := make(chan T, 1)
	go func() { ch <- f() }()
	select {
	case v := <-ch:
		return v
	case <-time.After(wait):
		t.Fatal("timed out")
		panic("unreachable")
	}
}

// take is the next value of ch, within wait.
func take[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	return within(t, func() T { return <-ch })
}

// peer is the gateway end of the next bridged stream.
func peer(t *testing.T, g *fakeGateway) *memPipe {
	t.Helper()
	return take(t, g.peers)
}

// nextFrame is the next frame p reads, within wait; a failed Read is a frame
// with its close reason.
func nextFrame(t *testing.T, p Pipe) frame {
	t.Helper()
	return within(t, func() frame {
		d, cols, rows, err := p.Read()
		if err != nil {
			reason := reasonOf(err)
			return frame{close: &reason}
		}
		return frame{data: d, cols: cols, rows: rows}
	})
}
