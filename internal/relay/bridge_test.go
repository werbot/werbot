package relay

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	relayv1 "github.com/werbot/werbot/gen/werbot/relay/v1"
	"github.com/werbot/werbot/gen/werbot/relay/v1/relayv1connect"
	"github.com/werbot/werbot/internal/platform/apperr"
)

func TestTerminalRemoteBridge(t *testing.T) {
	c := newCluster(t, 3, nil)
	b, bridge := c.rs[1], c.rs[2]
	g := startGateway(t, c, bridge)

	p, err := b.Terminal(context.Background(), nobody, "web1", 80, 24)
	require.NoError(t, err)
	gw := peer(t, g)
	assert.EqualValues(t, 1, streams(t, b))
	assert.EqualValues(t, 1, streams(t, bridge))
	assert.Zero(t, streams(t, c.rs[0]))

	require.NoError(t, p.Write([]byte("ls\r")))
	assert.Equal(t, "ls\r", readData(t, gw))
	require.NoError(t, p.Resize(100, 30))
	f := nextFrame(t, gw)
	assert.Equal(t, [2]int{100, 30}, [2]int{f.cols, f.rows})

	big := make([]byte, 1<<20)
	_, _ = rand.Read(big)
	require.NoError(t, gw.Write(big))
	var got []byte
	for len(got) < len(big) {
		got = append(got, readData(t, p)...)
	}
	assert.True(t, bytes.Equal(big, got), "1 MiB arrives whole and in order")

	require.NoError(t, p.Close(ReasonClosed))
	assert.Equal(t, ReasonClosed, readClose(t, gw))
	require.NoError(t, take(t, g.results))
	drained(t, b, bridge)
}

// TestCloseReasonCrossesReplicas: the gateway's close token reaches the
// browser replica unchanged ("limit" of the 17th observer becomes 4008 there).
func TestCloseReasonCrossesReplicas(t *testing.T) {
	c := newCluster(t, 2, nil)
	b, bridge := c.rs[0], c.rs[1]
	g := startGateway(t, c, bridge)
	for _, reason := range []string{ReasonDenied, ReasonNotFound, ReasonLimit, ReasonError} {
		p, err := b.Observe(context.Background(), nobody, uuid.New())
		require.NoError(t, err)
		gw := peer(t, g)
		require.NoError(t, gw.Close(reason))
		assert.Equal(t, reason, readClose(t, p))
		require.NoError(t, take(t, g.results))
	}
	drained(t, b, bridge)
}

func TestBridgeConsumesOnce(t *testing.T) {
	c := newCluster(t, 1, nil)
	b := c.rs[0]
	g := startGateway(t, c, b)
	p, err := b.Observe(context.Background(), nobody, uuid.New())
	require.NoError(t, err)
	id, gw := take(t, g.ids), peer(t, g)

	second := g.bridge(b, id)
	assert.Equal(t, ReasonNotFound, readClose(t, second), "a spent ticket opens nothing")
	require.NoError(t, take(t, g.results))
	assert.EqualValues(t, 1, rejected(t, b, "ticket"))

	require.NoError(t, gw.Write([]byte("first")))
	assert.Equal(t, "first", readData(t, p), "the browser keeps the first stream")
}

func TestBridgeRejectsForeignCaller(t *testing.T) {
	c := newCluster(t, 1, func(_ int, r *Relay) { r.awaitTimeout = 300 * time.Millisecond })
	b := c.rs[0]
	g := startGateway(t, c, nil)
	callers := map[string]context.Context{
		"another instance": WithCaller(context.Background(), c.tk.org, uuid.New()),
		"another org":      WithCaller(context.Background(), uuid.New(), c.tk.inst),
		"no caller":        context.Background(),
	}
	for name, ctx := range callers {
		t.Run(name, func(t *testing.T) {
			errc := make(chan error, 1)
			go func() { _, err := b.Observe(context.Background(), nobody, uuid.New()); errc <- err }()
			id := take(t, g.ids)
			gw, peer := memPair()
			require.NoError(t, b.Bridge(ctx, id, gw))
			assert.Equal(t, ReasonNotFound, readClose(t, peer))
			_, err := c.tk.Consume(context.Background(), id)
			assert.ErrorIs(t, err, apperr.ErrNotFound, "the ticket is spent")
			assert.ErrorIs(t, take(t, errc), ErrAwaitTimeout, "the browser's wait is untouched and expires")
		})
	}
	assert.EqualValues(t, len(callers), rejected(t, b, "ticket"))
	drained(t, b)
}

func TestBridgeTargetOffline(t *testing.T) {
	c := newCluster(t, 1, nil)
	bridge := c.rs[0]
	off := uuid.New()
	_, err := c.pool.Exec(context.Background(),
		"INSERT INTO server_instance (id, hostname, internal_url, last_seen_at) VALUES ($1, 'b', 'https://b:8444', now() - interval '1 minute')", off)
	require.NoError(t, err)
	id, _, _ := c.tk.add(off)
	gw, peer := memPair()
	require.NoError(t, bridge.Bridge(WithCaller(context.Background(), c.tk.org, c.tk.inst), id, gw))
	assert.Equal(t, ReasonError, readClose(t, peer))
	assert.EqualValues(t, 1, rejected(t, bridge, "unavailable"))
	drained(t, bridge)
}

// TestBridgeRefusesWrongHost: the bridge checks the replica's certificate
// against the host of its internal_url.
func TestBridgeRefusesWrongHost(t *testing.T) {
	c := newCluster(t, 2, nil)
	b, bridge := c.rs[0], c.rs[1]
	alias := uuid.New() // b's listener under a name its certificate lacks
	_, err := c.pool.Exec(context.Background(), "INSERT INTO server_instance (id, hostname, internal_url) VALUES ($1, 'b', $2)",
		alias, strings.Replace(b.cfg.InternalURL, "127.0.0.1", "localhost", 1))
	require.NoError(t, err)
	id, _, _ := c.tk.add(alias)
	gw, peer := memPair()
	require.NoError(t, bridge.Bridge(WithCaller(context.Background(), c.tk.org, c.tk.inst), id, gw))
	assert.Equal(t, ReasonError, readClose(t, peer))
	drained(t, bridge)
}

func TestStreamSlotsReleased(t *testing.T) {
	c := newCluster(t, 2, nil)
	b, bridge := c.rs[0], c.rs[1]
	g := startGateway(t, c, bridge)

	c.tk.err = errors.New("create failed")
	_, err := b.Observe(context.Background(), nobody, uuid.New())
	require.Error(t, err)
	c.tk.err = nil
	drained(t, b)

	ctx, cancel := context.WithCancel(context.Background())
	g.via.Store(nil)
	go func() { <-g.ids; cancel() }()
	_, err = b.Observe(ctx, nobody, uuid.New())
	require.ErrorIs(t, err, context.Canceled)
	drained(t, b)

	g.via.Store(bridge)
	p, err := b.Observe(context.Background(), nobody, uuid.New())
	require.NoError(t, err)
	take(t, g.ids)
	gw := peer(t, g)
	gw.fail.Store(true) // the gateway end refuses what the bridge writes to it
	require.NoError(t, p.Write([]byte("x")))
	assert.Equal(t, ReasonError, readClose(t, p))
	drained(t, b, bridge)
}

func TestPeerReplicaDies(t *testing.T) {
	var mu sync.Mutex
	var conns []net.Conn
	c := newCluster(t, 2, func(i int, r *Relay) {
		if i == 1 {
			r.tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				conn, err := (&net.Dialer{}).DialContext(ctx, network, addr)
				mu.Lock()
				conns = append(conns, conn)
				mu.Unlock()
				return conn, err
			}
		}
	})
	b, bridge := c.rs[0], c.rs[1]
	g := startGateway(t, c, bridge)
	p, err := b.Terminal(context.Background(), nobody, "web1", 80, 24)
	require.NoError(t, err)
	gw := peer(t, g)

	mu.Lock()
	for _, conn := range conns {
		_ = conn.Close() // the bridging replica is gone without a close frame
	}
	mu.Unlock()
	done := make(chan string, 1)
	go func() { done <- closeReason(p) }()
	select {
	case reason := <-done:
		assert.Empty(t, reason, "a lost peer is an error without a close reason")
	case <-time.After(15 * time.Second):
		t.Fatal("browser side not closed within 15s")
	}
	assert.Equal(t, ReasonError, readClose(t, gw))
	drained(t, b, bridge)
}

// pipeClient calls b's RelayService as the holder of cert.
func pipeClient(c *cluster, b *Relay, cert func(*tls.CertificateRequestInfo) (*tls.Certificate, error)) relayv1connect.RelayServiceClient {
	return relayv1connect.NewRelayServiceClient(client(c.ca, cert, true), b.cfg.InternalURL)
}

func TestPipeRefusesNonReplica(t *testing.T) {
	c := newCluster(t, 1, func(_ int, r *Relay) { r.awaitTimeout = 500 * time.Millisecond })
	b := c.rs[0]
	g := startGateway(t, c, nil)
	errc := make(chan error, 1)
	go func() { _, err := b.Observe(context.Background(), nobody, uuid.New()); errc <- err }()
	id := take(t, g.ids)

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	st := pipeClient(c, b, gatewayCert(t, c.ca)).Pipe(ctx)
	_ = st.Send(&relayv1.PipeRequest{Frame: &relayv1.PipeFrame{Frame: &relayv1.PipeFrame_TicketId{TicketId: id.String()}}})
	_, err := st.Receive()
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err), "a gateway certificate is not a replica")
	assert.ErrorIs(t, take(t, errc), ErrAwaitTimeout, "the waiting browser gets nothing")
}

func TestPipeHandsOnlyAwaitedStreams(t *testing.T) {
	c := newCluster(t, 1, nil)
	b := c.rs[0]
	rpc := pipeClient(c, b, b.clientCert)
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	st := rpc.Pipe(ctx)
	require.NoError(t, st.Send(&relayv1.PipeRequest{Frame: &relayv1.PipeFrame{Frame: &relayv1.PipeFrame_Data{Data: []byte("x")}}}))
	_, err := st.Receive()
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "the first frame names a ticket")

	st = rpc.Pipe(ctx)
	require.NoError(t, st.Send(&relayv1.PipeRequest{Frame: &relayv1.PipeFrame{Frame: &relayv1.PipeFrame_TicketId{TicketId: uuid.NewString()}}}))
	m, err := st.Receive()
	require.NoError(t, err)
	assert.Equal(t, ReasonNotFound, m.GetFrame().GetClose(), "no browser waits for the ticket")
}

func TestNoStreamContentInLogs(t *testing.T) {
	logs := &syncBuffer{}
	c := newCluster(t, 2, func(_ int, r *Relay) {
		r.log = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	})
	b, bridge := c.rs[0], c.rs[1]
	g := startGateway(t, c, bridge)
	p, err := b.Terminal(context.Background(), nobody, "web1", 80, 24)
	require.NoError(t, err)
	id, gw := take(t, g.ids), peer(t, g)
	require.NoError(t, p.Write([]byte("keystroke-marker")))
	assert.Equal(t, "keystroke-marker", readData(t, gw))
	require.NoError(t, gw.Write([]byte("output-marker")))
	assert.Equal(t, "output-marker", readData(t, p))
	require.NoError(t, gw.Close(ReasonClosed))
	assert.Equal(t, ReasonClosed, readClose(t, p))
	drained(t, b, bridge)

	out := logs.String()
	for _, secret := range []string{"keystroke-marker", "output-marker", id.String()} {
		assert.NotContains(t, out, secret)
	}
}
