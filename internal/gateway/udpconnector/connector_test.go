package udpconnector_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	_ "github.com/werbot/werbot/internal/connector/udp" // the udp resource type
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/connectortest"
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
	go func() {
		b := make([]byte, 65535)
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
	d      *dgrams
	reg    *metrics.Registry
	dials  atomic.Int32
	closed chan struct{} // one value per closed flow socket
	served chan error
	cancel context.CancelFunc
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
			c, err := dialer.DialContext(ctx, network, address)
			return closeNotify{c, h}, err
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

func TestContract(t *testing.T) {
	registered, ok := gateway.LookupConnector("udp")
	require.True(t, ok)
	require.Equal(t, udpconnector.Connector{}, registered)
	addr := echoUDP(t, "")
	connectortest.Run(t, udpconnector.Connector{}, connectortest.Target{
		Resource: resource(addr, ""), Secret: gateway.Secret{Kind: "none", Value: []byte("werbot-udp-secret")},
		Datagram: addr,
	})
}

func TestDialsTarget(t *testing.T) {
	var c gateway.Connector = udpconnector.Connector{}
	_, ok := c.(gateway.TargetDialer)
	require.True(t, ok)
}

func TestUDPFlowLimit(t *testing.T) {
	addr := echoUDP(t, "")
	d := newDgrams()
	h := start(t, addr, "", d)
	for f := byte(1); f <= 33; f++ {
		d.send(f, []byte{'a', f})
	}
	got := map[byte]bool{}
	for range 32 {
		f, p := d.recv(t)
		require.Equal(t, string([]byte{'a', f}), p)
		got[f] = true
	}
	require.Len(t, got, 32)
	require.False(t, got[33], "flow 33 was served")
	require.Eventually(t, func() bool { return h.metric(t, "flows") == "1" }, waitFor, 10*time.Millisecond)
	d.send(1, []byte("alive")) // the session lives
	f, p := d.recv(t)
	require.Equal(t, byte(1), f)
	require.Equal(t, "alive", p)
	h.cancel()
	h.mustServe(t)
}

// A flow without traffic for flow_idle loses its socket; its next datagram opens a new one.
func TestUDPFlowReopens(t *testing.T) {
	addr := echoUDP(t, "")
	d := newDgrams()
	h := start(t, addr, "1s", d)
	d.send(7, []byte("one"))
	_, p := d.recv(t)
	require.Equal(t, "one", p)
	select {
	case <-h.closed:
	case <-time.After(waitFor):
		t.Fatal("the idle flow was not closed")
	}
	d.send(7, []byte("two"))
	_, p = d.recv(t)
	require.Equal(t, "two", p)
	require.EqualValues(t, 2, h.dials.Load())
}

func TestUDPOversize(t *testing.T) {
	addr := echoUDP(t, "")
	d := newDgrams()
	h := start(t, addr, "", d)
	d.send(1, make([]byte, 9000))
	d.send(1, make([]byte, 100))
	_, p := d.recv(t)
	require.Len(t, p, 100)
	require.Equal(t, "1", h.metric(t, "size"))
}

func TestUDPPortUnreachable(t *testing.T) {
	probe, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	addr := probe.LocalAddr().String()
	require.NoError(t, probe.Close()) // a closed port
	d := newDgrams()
	h := start(t, addr, "", d)
	d.send(1, []byte("lost"))
	require.Eventually(t, func() bool { return h.dials.Load() == 1 }, waitFor, 5*time.Millisecond)
	echoUDP(t, addr) // the target starts
	got := make(chan string, 1)
	require.Eventually(t, func() bool {
		d.send(1, []byte("hello"))
		select {
		case m := <-d.out:
			got <- string(m[1:])
			return true
		case <-time.After(50 * time.Millisecond):
			return false
		}
	}, waitFor, 10*time.Millisecond)
	require.Equal(t, "hello", <-got)
	select {
	case err := <-h.served:
		t.Fatalf("session ended: %v", err)
	default:
	}
}

// A client that stops reading blocks Send; Serve still returns on cancel and the Recv loop keeps going.
func TestUDPBlockedSend(t *testing.T) {
	addr := echoUDP(t, "")
	d := newDgrams()
	d.block = true
	h := start(t, addr, "", d)
	d.send(1, []byte("a"))
	for i := range 3 { // Recv still takes datagrams of other flows while a Send is stuck
		d.send(byte(2+i), []byte("b"))
	}
	require.Eventually(t, func() bool { return len(d.in) == 0 }, waitFor, 5*time.Millisecond)
	h.cancel()
	h.mustServe(t)
}

func TestUDPMemory(t *testing.T) {
	addr := echoUDP(t, "")
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	d := newDgrams()
	h := start(t, addr, "", d)
	for f := byte(1); f <= 32; f++ {
		d.send(f, []byte("x"))
	}
	for range 32 {
		d.recv(t)
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	grown := int64(after.HeapAlloc+after.StackInuse) - int64(before.HeapAlloc+before.StackInuse) //nolint:gosec // far below MaxInt64
	t.Logf("growth with 32 flows: %d bytes", grown)
	require.Less(t, grown, int64(300<<10))
	h.cancel()
	h.mustServe(t)
}

type panicDgrams struct{ *dgrams }

func (panicDgrams) Recv([]byte) (uint32, int, error) { panic("fault injected") }

func TestUDPPanic(t *testing.T) {
	s := &gateway.Session{Resource: resource("127.0.0.1:9", ""), Datagram: panicDgrams{newDgrams()}, Target: "127.0.0.1:9",
		Secret: gateway.Secret{Value: []byte("udp-secret-value")}}
	var p any
	func() {
		defer func() { p = recover() }()
		_ = udpconnector.Connector{}.Serve(t.Context(), s)
	}()
	require.Contains(t, fmt.Sprint(p), "fault injected")
}

// sink is a minimal http.ResponseWriter for reading the metrics text.
type sink struct{ b *strings.Builder }

func (s *sink) Header() http.Header         { return http.Header{} }
func (s *sink) Write(p []byte) (int, error) { return s.b.Write(p) }
func (s *sink) WriteHeader(int)             {}
