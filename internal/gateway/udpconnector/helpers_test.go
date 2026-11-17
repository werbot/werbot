package udpconnector_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/udpconnector"
	"github.com/werbot/werbot/internal/platform/metrics"
)

const waitFor = 5 * time.Second

// echoUDP starts a UDP echo target on addr ("" is a free loopback port).
func echoUDP(t *testing.T, addr string) string {
	t.Helper()
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	ua, err := net.ResolveUDPAddr("udp", addr)
	require.NoError(t, err)
	c, err := net.ListenUDP("udp", ua)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	b := make([]byte, 65535)
	go func() {
		for {
			n, from, err := c.ReadFromUDP(b)
			if err != nil {
				return
			}
			_, _ = c.WriteToUDP(b[:n], from)
		}
	}()
	return c.LocalAddr().String()
}

func resource(addr, flowIdle string) gateway.Resource {
	host, port, _ := net.SplitHostPort(addr)
	cfg := fmt.Sprintf(`{"host":%q,"ports":[{"port":%s}]`, host, port)
	if flowIdle != "" {
		cfg += fmt.Sprintf(`,"flowIdle":%q`, flowIdle)
	}
	return gateway.Resource{ID: uuid.New(), Name: "svc", Type: "udp", Config: []byte(cfg + "}")}
}

// dgrams is a Datagrams like the SSH channel's: a datagram larger than buf is dropped with io.ErrShortBuffer.
type dgrams struct {
	in, out chan []byte // in: flow(4 bytes) + payload from the client
	done    chan struct{}
	once    sync.Once
	block   bool // Send never returns until Close: a client that stopped reading
}

func newDgrams() *dgrams {
	return &dgrams{in: make(chan []byte, 64), out: make(chan []byte, 64), done: make(chan struct{})}
}

func (d *dgrams) Recv(buf []byte) (uint32, int, error) {
	select {
	case m := <-d.in:
		if len(m)-1 > len(buf) {
			return uint32(m[0]), 0, io.ErrShortBuffer
		}
		return uint32(m[0]), copy(buf, m[1:]), nil
	case <-d.done:
		return 0, 0, net.ErrClosed
	}
}

func (d *dgrams) Send(flow uint32, p []byte) error {
	if d.block {
		<-d.done
		return net.ErrClosed
	}
	select {
	case d.out <- append([]byte{byte(flow)}, p...): //nolint:gosec // test flows are below 256
		return nil
	case <-d.done:
		return net.ErrClosed
	}
}

func (d *dgrams) Close() error { d.once.Do(func() { close(d.done) }); return nil }

func (d *dgrams) send(flow byte, p []byte) { d.in <- append([]byte{flow}, p...) }

// recv returns the next datagram sent to the client as flow, payload.
func (d *dgrams) recv(t *testing.T) (byte, string) {
	t.Helper()
	select {
	case m := <-d.out:
		return m[0], string(m[1:])
	case <-time.After(waitFor):
		t.Fatal("no datagram within the wait")
		return 0, ""
	}
}

type harness struct {
	d       *dgrams
	reg     *metrics.Registry
	dials   atomic.Int32
	dialErr atomic.Pointer[error] // set: dials fail with it
	closed  chan struct{}         // one value per closed flow socket
	served  chan error
	cancel  context.CancelFunc
}

type closeNotify struct {
	net.Conn
	h *harness
}

func (c closeNotify) Close() error { c.h.closed <- struct{}{}; return c.Conn.Close() }

// start serves a session against addr; the client side is h.d.
func start(t *testing.T, addr, flowIdle string, d *dgrams) *harness {
	t.Helper()
	h := &harness{d: d, reg: metrics.NewRegistry(), closed: make(chan struct{}, 1024), served: make(chan error, 1)}
	dialer := &net.Dialer{Timeout: waitFor}
	s := &gateway.Session{
		ID: uuid.New(), Kind: gateway.KindDatagram, Resource: resource(addr, flowIdle), Datagram: d, Target: addr,
		Secret: gateway.Secret{Kind: "none", Value: []byte("udp-secret-value")}, Metrics: h.reg,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			h.dials.Add(1)
			if e := h.dialErr.Load(); e != nil {
				return nil, *e
			}
			c, err := dialer.DialContext(ctx, network, address)
			if err != nil {
				return nil, err
			}
			return closeNotify{c, h}, nil
		},
	}
	var ctx context.Context
	ctx, h.cancel = context.WithCancel(t.Context())
	t.Cleanup(h.cancel)
	go func() { h.served <- udpconnector.Connector{}.Serve(ctx, s) }()
	return h
}

func (h *harness) metric(t *testing.T, reason string) string {
	t.Helper()
	var b strings.Builder
	h.reg.Handler().ServeHTTP(&sink{&b}, httptest.NewRequest("GET", "/metrics", nil))
	for l := range strings.SplitSeq(b.String(), "\n") {
		if strings.HasPrefix(l, `werbot_gateway_datagrams_dropped_total{reason="`+reason+`"}`) {
			return strings.Fields(l)[1]
		}
	}
	return ""
}

func (h *harness) mustServe(t *testing.T) {
	t.Helper()
	select {
	case err := <-h.served:
		require.NoError(t, err)
	case <-time.After(waitFor):
		t.Fatal("Serve did not return")
	}
}

// sink is a minimal http.ResponseWriter for reading the metrics text.
type sink struct{ b *strings.Builder }

func (s *sink) Header() http.Header         { return http.Header{} }
func (s *sink) Write(p []byte) (int, error) { return s.b.Write(p) }
func (s *sink) WriteHeader(int)             {}

// bigUDP starts a target that answers every datagram with 9000 bytes, then 100 bytes.
func bigUDP(t *testing.T) string {
	t.Helper()
	c, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	go func() {
		b := make([]byte, 64)
		for {
			_, from, err := c.ReadFromUDP(b)
			if err != nil {
				return
			}
			_, _ = c.WriteToUDP(make([]byte, 9000), from)
			_, _ = c.WriteToUDP(make([]byte, 100), from)
		}
	}()
	return c.LocalAddr().String()
}

// finish cancels the session and waits for Serve to return.
func (h *harness) finish(t *testing.T) {
	t.Helper()
	h.cancel()
	h.mustServe(t)
}

// failDial makes dials fail with err; nil makes them work again.
func (h *harness) failDial(err error) {
	if err == nil {
		h.dialErr.Store(nil)
		return
	}
	h.dialErr.Store(&err)
}
