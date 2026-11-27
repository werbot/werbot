package pgconnector

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/dbproxy"
)

// Protocol relay limits.
const (
	maxParsed   = 16 << 20 // Query, Parse, Bind, Execute, FunctionCall and Close read whole
	maxNames    = 10000    // prepared statements plus portals of one connection
	maxRetained = 16 << 20 // bytes of names, logged texts and parameters they keep
	maxReply    = 1 << 20  // CommandComplete, ErrorResponse and ReadyForQuery read whole
	keepBuf     = 64 << 10 // a message buffer above this is released after its message
)

var (
	errMalformed    = errors.New("postgres: malformed protocol message")
	errTooLarge     = errors.New("postgres: client message exceeds 16 MiB")
	errTooManyNames = errors.New("postgres: too many prepared statements")
	errRetained     = errors.New("postgres: prepared statements and portals exceed 16 MiB")
	errDenied       = errors.New("postgres: operation denied")
	errTerminate    = errors.New("postgres: client terminated")
)

// fatalText is what the client reads before the gateway ends its session.
var fatalText = map[error]string{
	errTooLarge:     "werbot: message exceeds 16 MiB",
	errTooManyNames: "werbot: too many prepared statements",
	errRetained:     "werbot: prepared statements and portals exceed 16 MiB",
}

// sessionLog is what the relay stamps on the querylog lines of a session.
type sessionLog struct {
	t0     time.Time // T of a line counts from here
	db     string
	params bool // record_params
	secret gateway.Secret
}

// newRelay builds both directions of a session over one queue of the replies
// the client expects: cr and client are the client stream, db the database
// connection.
func newRelay(ctx context.Context, cr *bufio.Reader, db io.ReadWriter, client io.Writer, g *dbproxy.Guard, l sessionLog) (*clientSide, *dbSide) {
	q := &queue{}
	out := &lockedWriter{w: client}
	c := &clientSide{ctx: ctx, r: cr, w: bufio.NewWriterSize(db, ioBuf), out: out, q: q, g: g, log: l,
		stmts: map[string]string{}, portals: map[string]portal{}}
	d := &dbSide{r: bufio.NewReaderSize(db, ioBuf), w: bufio.NewWriterSize(out, ioBuf), q: q, g: g, secret: l.secret}
	return c, d
}

// relay runs the session both ways without waiting for replies (pipelining
// is kept); the first end closes both sides. After a Terminate the database
// ends the connection itself, so queries in flight still get their lines.
func relay(ctx context.Context, s *gateway.Session, hc *pgconn.HijackedConn, c *clientSide, d *dbSide) error {
	sctx, stop := context.WithCancel(ctx)
	defer stop()
	closeBoth := func() { _ = s.Stream.Close(); _ = hc.Conn.Close() }
	defer context.AfterFunc(sctx, closeBoth)()
	g := gateway.NewGuard(stop, s.Secret)
	g.Go(func() error {
		err := run(c.next)
		if !errors.Is(err, errTerminate) {
			stop()
		}
		return endErr(err)
	})
	g.Go(func() error { defer stop(); return endErr(run(d.next)) })
	err := g.Wait()
	closeBoth()
	if ctx.Err() != nil {
		return nil
	}
	return err
}

// run calls next until it fails.
func run(next func() error) error {
	for {
		if err := next(); err != nil {
			return err
		}
	}
}

// endErr drops the errors of a normal end: a Terminate, a closed side.
func endErr(err error) error {
	if errors.Is(err, errTerminate) || gateway.IsClosed(err) {
		return nil
	}
	return err
}

// readClient reads the header of the next client message and, for Query,
// Parse, Bind, Execute, FunctionCall and Close within maxParsed, its body into
// buf (grown when short). body == nil: the n body bytes are still in r, to be
// streamed. A parsed type other than Bind over maxParsed is errTooLarge.
func readClient(r *bufio.Reader, buf []byte) (typ byte, body []byte, n int, err error) {
	h, err := r.Peek(5)
	if err != nil {
		return 0, nil, 0, err
	}
	typ, size := h[0], int(int32(binary.BigEndian.Uint32(h[1:]))) //nolint:gosec // a signed length word
	if size < 4 {
		return 0, nil, 0, errMalformed
	}
	_, _ = r.Discard(5)
	n = size - 4
	switch {
	case strings.IndexByte("QPBEFC", typ) < 0:
		return typ, nil, n, nil
	case n > maxParsed && typ == 'B':
		return typ, nil, n, nil
	case n > maxParsed:
		return typ, nil, n, errTooLarge
	}
	if buf == nil || cap(buf) < n {
		buf = make([]byte, n)
	}
	body = buf[:n]
	if _, err := io.ReadFull(r, body); err != nil {
		return 0, nil, 0, err
	}
	return typ, body, n, nil
}

// pass copies the next n bytes of r to w through lr, never holding them whole.
func pass(w *bufio.Writer, lr *io.LimitedReader, r io.Reader, n int) error {
	lr.R, lr.N = r, int64(n)
	m, err := w.ReadFrom(lr)
	if err == nil && m < int64(n) {
		err = io.ErrUnexpectedEOF
	}
	return err
}

// lockedWriter serializes the writes to the client stream: the database side
// flushes replies, the client side may end the session with a FATAL.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

// sendLimit tells the client which limit ended its session.
// ponytail: a FATAL may land between two partial flushes of a reply larger
// than the write buffer; the session ends right after either way.
func sendLimit(w io.Writer, err error) {
	sendFatal(w, &pgproto3.ErrorResponse{Severity: "FATAL", SeverityUnlocalized: "FATAL", Code: "54000", Message: fatalText[err]})
}
