package gateway

import (
	"bytes"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/metrics"
)

// runStream opens a stream to target on "pg" and runs it on echo; it returns the
// client end of the stream and a channel closed when Run returned.
func (e *pipeEnv) runStream(t *testing.T, target string) (*Session, net.Conn, <-chan struct{}) {
	t.Helper()
	s, err := e.p.Open(t.Context(), e.target(KindStream, target))
	require.NoError(t, err)
	client, server := net.Pipe()
	s.Stream = server
	c, _ := LookupConnector("plainstream")
	done := make(chan struct{})
	go func() { defer close(done); _ = e.p.Run(t.Context(), s, c) }()
	t.Cleanup(func() { _ = client.Close(); <-done })
	return s, client, done
}

// roundTrip sends msg through an echo stream and reads it back.
func roundTrip(t *testing.T, c net.Conn, msg []byte) {
	t.Helper()
	errc := make(chan error, 1)
	go func() { _, err := c.Write(msg); errc <- err }()
	got := make([]byte, len(msg))
	_, err := io.ReadFull(c, got)
	require.NoError(t, err)
	require.NoError(t, <-errc)
	require.Equal(t, msg, got)
}

// endCounters is the counters the delivered EndSession of id carried.
func (f *fakeControl) endCounters(id uuid.UUID) *gatewayv1.SessionCounters {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.ends {
		if e.id == id {
			return e.counters
		}
	}
	return nil
}

func TestCountersBothDirections(t *testing.T) {
	e := newPlainEnv(t)
	s, client, done := e.runStream(t, "pg:5432")
	roundTrip(t, client, bytes.Repeat([]byte("x"), 1000))
	require.NoError(t, client.Close())
	<-done
	requireEnds(t, e.f, s.ID, "closed")
	want := &gatewayv1.SessionCounters{SessionId: s.ID.String(), BytesUp: 1000, BytesDown: 1000}
	require.True(t, proto.Equal(want, e.f.endCounters(s.ID)), "%v", e.f.endCounters(s.ID))
	m := e.metrics(t)
	require.Contains(t, m, `werbot_gateway_session_bytes_total{direction="up"} 1000`)
	require.Contains(t, m, `werbot_gateway_session_bytes_total{direction="down"} 1000`)
}

func TestHeartbeatCountersEveryFourth(t *testing.T) {
	e := newPlainEnv(t)
	a, ca, _ := e.runStream(t, "pg:5432")
	b, cb, _ := e.runStream(t, "pg:5432")
	roundTrip(t, ca, []byte("hi"))
	for i := 1; i <= 8; i++ {
		if i == 5 {
			roundTrip(t, cb, []byte("hello"))
		}
		hb := e.p.heartbeat()
		requireValid(t, hb)
		switch i {
		case 4:
			require.Len(t, hb.GetCounters(), 1, "b has no traffic yet")
			require.True(t, proto.Equal(&gatewayv1.SessionCounters{SessionId: a.ID.String(), BytesUp: 2, BytesDown: 2}, hb.GetCounters()[0]))
		case 8:
			require.Len(t, hb.GetCounters(), 1, "a did not change")
			require.True(t, proto.Equal(&gatewayv1.SessionCounters{SessionId: b.ID.String(), BytesUp: 5, BytesDown: 5}, hb.GetCounters()[0]))
		default:
			require.Empty(t, hb.GetCounters(), i)
		}
	}
}

type plainRWC struct{ io.ReadWriteCloser }

type halfCloser struct{ plainRWC }

func (halfCloser) CloseWrite() error { return nil }

type fakeDatagrams struct{}

func (fakeDatagrams) Recv(buf []byte) (uint32, int, error) { return 7, copy(buf, "abc"), nil }
func (fakeDatagrams) Send(uint32, []byte) error            { return nil }
func (fakeDatagrams) Close() error                         { return nil }

// The wrappers count every kind and keep CloseWrite only where the stream has it.
func TestCountersWrap(t *testing.T) {
	start := time.Now().Add(-time.Hour)
	c := newCounters(start)
	m := meter{c, newMeters(metrics.NewRegistry())}

	s := &Session{Stream: plainRWC{}}
	m.wrap(s)
	_, ok := s.Stream.(closeWriter)
	require.False(t, ok)
	s = &Session{Stream: halfCloser{}}
	m.wrap(s)
	_, ok = s.Stream.(closeWriter)
	require.True(t, ok)

	var out bytes.Buffer
	s = &Session{Exec: &Exec{Stdin: strings.NewReader("abc"), Stdout: &out, Stderr: &out}, Datagram: fakeDatagrams{}}
	m.wrap(s)
	_, err := io.ReadAll(s.Exec.Stdin)
	require.NoError(t, err)
	_, _ = io.WriteString(s.Exec.Stdout, "de")
	_, _ = io.WriteString(s.Exec.Stderr, "f")
	buf := make([]byte, 16)
	flow, n, err := s.Datagram.Recv(buf)
	require.NoError(t, err)
	require.Equal(t, uint32(7), flow)
	require.NoError(t, s.Datagram.Send(flow, buf[:n+2]))
	require.Equal(t, [4]int64{6, 8, 1, 1}, c.load())
	require.Less(t, c.idleFor(time.Now()), time.Minute, "traffic restarts the idle clock")
	require.Equal(t, time.Hour, c.idleFor(start.Add(time.Hour+time.Duration(c.last.Load()))), "monotonic offsets from the start")
}
