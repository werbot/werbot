package relay

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/pgnotify"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/platform/tlsca"
)

// fakeTickets issues tickets of one organization and gateway instance and
// spends each once, like session's ConsumeStreamTicket.
type fakeTickets struct {
	org, inst uuid.UUID
	mu        sync.Mutex
	m         map[uuid.UUID]Ticket
	err       error // Create* fails with it when set
}

func (f *fakeTickets) add(target uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return uuid.Nil, uuid.Nil, f.err
	}
	id := uuid.New()
	f.m[id] = Ticket{ID: id, OrgID: f.org, InstanceID: f.inst, TargetServerID: target}
	return id, f.inst, nil
}

func (f *fakeTickets) CreateObserve(_ context.Context, _ principal.Principal, _, target uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	return f.add(target)
}

func (f *fakeTickets) CreateTerminal(_ context.Context, _ principal.Principal, _ string, _, _ int, target uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	return f.add(target)
}

func (f *fakeTickets) Consume(_ context.Context, id uuid.UUID) (Ticket, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.m[id]
	if !ok {
		return Ticket{}, apperr.ErrNotFound
	}
	delete(f.m, id)
	return t, nil
}

// frame is one message of a memPipe; close != nil closes the stream.
type frame struct {
	data       []byte
	cols, rows int
	close      *string
}

// memPipe is one end of an in-memory stream; end tears the stream down like
// a finished RPC: pending frames are still read, then Read fails.
type memPipe struct {
	in, out chan frame
	gone    chan struct{}
	end     func()
	fail    *atomic.Bool // set: every write of both ends fails
	mu      sync.Mutex
	closed  bool
}

func memPair() (*memPipe, *memPipe) {
	ab, ba, gone, fail := make(chan frame, 64), make(chan frame, 64), make(chan struct{}), &atomic.Bool{}
	end := sync.OnceFunc(func() { close(gone) })
	return &memPipe{in: ba, out: ab, gone: gone, end: end, fail: fail}, &memPipe{in: ab, out: ba, gone: gone, end: end, fail: fail}
}

func (p *memPipe) Read() ([]byte, int, int, error) {
	var f frame
	select {
	case f = <-p.in:
	default:
		select {
		case f = <-p.in:
		case <-p.gone:
			return nil, 0, 0, io.ErrUnexpectedEOF
		}
	}
	if f.close != nil {
		return nil, 0, 0, CloseError{Reason: *f.close}
	}
	return f.data, f.cols, f.rows, nil
}

func (p *memPipe) put(f frame) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.fail.Load() {
		return errPipeClosed
	}
	p.closed = f.close != nil
	select {
	case p.out <- f:
		return nil
	case <-p.gone:
		return io.ErrClosedPipe
	}
}

func (p *memPipe) Write(b []byte) error        { return p.put(frame{data: b}) }
func (p *memPipe) Resize(cols, rows int) error { return p.put(frame{cols: cols, rows: rows}) }
func (p *memPipe) Close(reason string) error {
	if err := p.put(frame{close: &reason}); err != nil && !errors.Is(err, errPipeClosed) {
		return err
	}
	return nil
}

// fakeGateway hears werbot_stream_open of its instance, records the ticket
// ids and, when via is set, bridges each on via as its own caller.
type fakeGateway struct {
	tk      *fakeTickets
	via     atomic.Pointer[Relay]
	ids     chan uuid.UUID
	peers   chan *memPipe // the gateway ends of bridged streams
	results chan error    // what Bridge returned
}

func startGateway(t *testing.T, c *cluster, via *Relay) *fakeGateway {
	t.Helper()
	g := &fakeGateway{tk: c.tk, ids: make(chan uuid.UUID, 16), peers: make(chan *memPipe, 16), results: make(chan error, 16)}
	g.via.Store(via)
	ctx, cancel := context.WithCancel(context.Background())
	ready, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		_ = pgnotify.Listen(ctx, c.pool, map[string]func(string){pgnotify.StreamOpen: func(payload string) {
			inst, ticket, _ := strings.Cut(payload, ":")
			id, err := uuid.Parse(ticket)
			if inst != g.tk.inst.String() || err != nil {
				t.Errorf("stream open payload of another instance or malformed")
				return
			}
			g.ids <- id
			if via := g.via.Load(); via != nil {
				g.bridge(via, id)
			}
		}}, sync.OnceFunc(func() { close(ready) }), func() {})
	}()
	t.Cleanup(func() { cancel(); <-done })
	<-ready
	return g
}

// bridge runs Bridge on r for ticket id as the ticket's own gateway and
// returns the gateway end; the stream ends when Bridge returns.
func (g *fakeGateway) bridge(r *Relay, id uuid.UUID) *memPipe {
	gw, peer := memPair()
	g.peers <- peer
	go func() {
		err := r.Bridge(WithCaller(context.Background(), g.tk.org, g.tk.inst), id, gw)
		gw.end()
		g.results <- err
	}()
	return peer
}

// cluster is n started replicas on one database sharing a CA and tickets;
// tune runs on each before it starts.
type cluster struct {
	pool  *pgxpool.Pool
	ca    *tlsca.CA
	tk    *fakeTickets
	rs    []*Relay
	stops []func() error
}

func newCluster(t *testing.T, n int, tune func(i int, r *Relay)) *cluster {
	t.Helper()
	pool, _ := dbtest.New(t)
	c := &cluster{pool: pool, ca: newCA(t), tk: &fakeTickets{org: uuid.New(), inst: uuid.New(), m: map[uuid.UUID]Ticket{}}}
	for i := range n {
		r := newRelay(t, pool, c.ca)
		r.tickets = c.tk
		if tune != nil {
			tune(i, r)
		}
		c.rs = append(c.rs, r)
		c.stops = append(c.stops, start(t, r))
	}
	return c
}

// metric is the value of one series of r's registry; 0 when absent.
func metric(t *testing.T, r *Relay, series string) int64 {
	t.Helper()
	rec := httptest.NewRecorder()
	r.reg.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	for line := range strings.Lines(rec.Body.String()) {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), series+" "); ok {
			n, err := strconv.ParseInt(v, 10, 64)
			require.NoError(t, err)
			return n
		}
	}
	return 0
}

func streams(t *testing.T, r *Relay) int64 { return metric(t, r, "werbot_relay_streams") }

func rejected(t *testing.T, r *Relay, reason string) int64 {
	return metric(t, r, `werbot_relay_rejected_total{reason="`+reason+`"}`)
}

// readData is the next frame of p, within wait; it must be data.
func readData(t *testing.T, p Pipe) string {
	t.Helper()
	f := nextFrame(t, p)
	require.Nil(t, f.close, "want data, got the end of the stream")
	require.Zero(t, f.cols, "want data, got a resize")
	return string(f.data)
}

// readClose reads p until it fails, within wait, and returns the close
// reason, "" when the stream ended without one.
func readClose(t *testing.T, p Pipe) string {
	t.Helper()
	return within(t, func() string { return closeReason(p) })
}

// closeReason reads p until it fails and returns the close reason, "" when
// the stream ended without one.
func closeReason(p Pipe) string {
	for {
		_, _, _, err := p.Read()
		if err == nil {
			continue
		}
		var r interface{ CloseReason() string }
		if errors.As(err, &r) {
			return r.CloseReason()
		}
		return ""
	}
}

func drained(t *testing.T, rs ...*Relay) {
	t.Helper()
	for _, r := range rs {
		require.Eventually(t, func() bool { return streams(t, r) == 0 }, 5*time.Second, 10*time.Millisecond, "every slot released")
	}
}

// syncBuffer is a log sink safe for concurrent handlers.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}
