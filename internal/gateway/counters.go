package gateway

import (
	"io"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/metrics"
)

// counters is the traffic of one session: up is client to target, down target to
// client; last is the time of the last traffic in either direction, in nanoseconds
// since started on the monotonic clock, so a wall clock step does not idle sessions.
type counters struct {
	bytesUp, bytesDown, dgUp, dgDown, last atomic.Int64
	started                                time.Time
}

func newCounters(started time.Time) *counters { return &counters{started: started} }

// touch records activity now.
func (c *counters) touch() { c.last.Store(int64(time.Since(c.started))) }

// idleFor is how long the session has had no traffic at now.
func (c *counters) idleFor(now time.Time) time.Duration {
	return now.Sub(c.started) - time.Duration(c.last.Load())
}

// load reads bytes up, bytes down, datagrams up, datagrams down.
func (c *counters) load() [4]int64 {
	return [4]int64{c.bytesUp.Load(), c.bytesDown.Load(), c.dgUp.Load(), c.dgDown.Load()}
}

// countersProto is the wire form of the counters v of session id.
func countersProto(id uuid.UUID, v [4]int64) *gatewayv1.SessionCounters {
	return &gatewayv1.SessionCounters{SessionId: id.String(), BytesUp: v[0], BytesDown: v[1], DatagramsUp: v[2], DatagramsDown: v[3]}
}

// meters are the instance totals behind the per-session counters.
type meters struct{ bytesUp, bytesDown, dgUp, dgDown *metrics.Counter }

func newMeters(reg *metrics.Registry) meters {
	const bh, dh = "Session payload bytes, by direction (up: client to target).", "Session datagrams, by direction (up: client to target)."
	return meters{
		bytesUp:   reg.Counter("werbot_gateway_session_bytes_total", bh, "direction", "up"),
		bytesDown: reg.Counter("werbot_gateway_session_bytes_total", bh, "direction", "down"),
		dgUp:      reg.Counter("werbot_gateway_datagrams_total", dh, "direction", "up"),
		dgDown:    reg.Counter("werbot_gateway_datagrams_total", dh, "direction", "down"),
	}
}

// meter counts the traffic of one session.
type meter struct {
	c *counters
	m meters
}

func (m meter) up(n int) {
	if n > 0 {
		m.c.bytesUp.Add(int64(n))
		m.m.bytesUp.Add(uint64(n))
		m.touch()
	}
}

func (m meter) down(n int) {
	if n > 0 {
		m.c.bytesDown.Add(int64(n))
		m.m.bytesDown.Add(uint64(n))
		m.touch()
	}
}

// touch notes traffic now: the idle timeout restarts.
func (m meter) touch() { m.c.touch() }

// wrap makes the session's client side count its traffic; Run calls it before
// Serve, once the front has set the streams.
func (m meter) wrap(s *Session) {
	if t := s.Terminal; t != nil {
		t.ReadWriter = countRW{t.ReadWriter, m}
	}
	if e := s.Exec; e != nil {
		e.Stdin = countReader{e.Stdin, m}
		e.Stdout, e.Stderr = countWriter{e.Stdout, m}, countWriter{e.Stderr, m}
	}
	if st := s.Stream; st != nil {
		s.Stream = m.stream(st)
	}
	if d := s.Datagram; d != nil {
		s.Datagram = countDatagrams{d, m}
	}
	if rf := s.RemoteForward; rf != nil {
		open := rf.Open
		rf.Open = func(addr string, port uint32) (io.ReadWriteCloser, error) {
			st, err := open(addr, port)
			if err != nil {
				return nil, err
			}
			return m.stream(st), nil
		}
	}
}

// stream counts the traffic of st and keeps its CloseWrite.
func (m meter) stream(st io.ReadWriteCloser) io.ReadWriteCloser {
	if cw, ok := st.(closeWriter); ok {
		return countStreamCW{countStream{countRW{st, m}, st}, cw}
	}
	return countStream{countRW{st, m}, st}
}

type closeWriter interface{ CloseWrite() error }

type countRW struct {
	io.ReadWriter
	m meter
}

func (c countRW) Read(p []byte) (int, error) {
	n, err := c.ReadWriter.Read(p)
	c.m.up(n)
	return n, err
}

func (c countRW) Write(p []byte) (int, error) {
	n, err := c.ReadWriter.Write(p)
	c.m.down(n)
	return n, err
}

type countReader struct {
	io.Reader
	m meter
}

func (c countReader) Read(p []byte) (int, error) {
	n, err := c.Reader.Read(p)
	c.m.up(n)
	return n, err
}

type countWriter struct {
	io.Writer
	m meter
}

func (c countWriter) Write(p []byte) (int, error) {
	n, err := c.Writer.Write(p)
	c.m.down(n)
	return n, err
}

// countStream hides CloseWrite; countStreamCW keeps it for a stream that has it.
type countStream struct {
	countRW
	io.Closer
}

type countStreamCW struct {
	countStream
	closeWriter
}

type countDatagrams struct {
	Datagrams
	m meter
}

func (c countDatagrams) Recv(buf []byte) (uint32, int, error) {
	flow, n, err := c.Datagrams.Recv(buf)
	if err == nil {
		c.m.c.dgUp.Add(1)
		c.m.m.dgUp.Inc()
		c.m.touch()
		c.m.up(n)
	}
	return flow, n, err
}

func (c countDatagrams) Send(flow uint32, p []byte) error {
	err := c.Datagrams.Send(flow, p)
	if err == nil {
		c.m.c.dgDown.Add(1)
		c.m.m.dgDown.Inc()
		c.m.touch()
		c.m.down(len(p))
	}
	return err
}
