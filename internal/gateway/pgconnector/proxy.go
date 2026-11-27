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
	maxPending  = 65536    // client messages of one connection that wait for a reply
	maxReply    = 1 << 20  // CommandComplete, ErrorResponse and ReadyForQuery read whole
	keepBuf     = 64 << 10 // a message buffer above this is released after its message
)

// limitErr is a broken relay limit: the client reads "werbot: <msg>" in a
// FATAL 54000 before the gateway ends its session.
type limitErr struct{ msg string }

func (e *limitErr) Error() string { return "postgres: " + e.msg }

var (
	errTooLarge     = &limitErr{"message exceeds 16 MiB"}
	errTooManyNames = &limitErr{"too many prepared statements"}
	errRetained     = &limitErr{"prepared statements and portals exceed 16 MiB"}
	errPending      = &limitErr{"too many pending messages"}
	errMalformed    = errors.New("postgres: malformed protocol message")
	errTerminate    = errors.New("postgres: client terminated")
)

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
	out := &clientWriter{w: bufio.NewWriterSize(client, ioBuf)}
	c := &clientSide{ctx: ctx, r: cr, w: bufio.NewWriterSize(db, ioBuf), out: out, q: q, g: g, log: l,
		stmts: map[string]named{}, portals: map[string]named{}}
	d := &dbSide{r: bufio.NewReaderSize(db, ioBuf), w: out.w, out: out, q: q, g: g, sess: l, lex: &c.lex, resets: &c.resets}
	return c, d
}

// relay runs the session both ways; a person's messages go on without
// waiting for replies (pipelining is kept). The first end closes both sides.
// After a Terminate the database ends the connection itself, so queries in
// flight still get their lines.
func relay(ctx context.Context, s *gateway.Session, hc *pgconn.HijackedConn, c *clientSide, d *dbSide) error {
	sctx, stop := context.WithCancel(ctx)
	defer stop()
	c.ctx = sctx // an approval or an agent's wait ends with the session
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
	if typ, n, err = readHeader(r); err != nil {
		return 0, nil, 0, err
	}
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

// readHeader reads the header of the next message of r: its type and the
// length of its body.
func readHeader(r *bufio.Reader) (typ byte, n int, err error) {
	h, err := r.Peek(5)
	if err != nil {
		return 0, 0, err
	}
	typ, size := h[0], int(int32(binary.BigEndian.Uint32(h[1:]))) //nolint:gosec // a signed length word
	if size < 4 {
		return 0, 0, errMalformed
	}
	_, _ = r.Discard(5)
	return typ, size - 4, nil
}

// appendHeader appends the header of a message of type typ with n body bytes.
func appendHeader(b []byte, typ byte, n int) []byte {
	return binary.BigEndian.AppendUint32(append(b, typ), uint32(n+4)) //nolint:gosec // n came from a 32-bit length
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

// clientWriter is the client stream behind one lock: the database side
// holds it for each whole reply message, the client side for each message it
// writes itself (a notice, a denial, a FATAL), so no message lands inside
// another. status is the transaction status of the last ReadyForQuery the
// client got.
type clientWriter struct {
	mu     sync.Mutex
	w      *bufio.Writer
	status byte
}

// send writes the whole messages b and flushes them.
func (o *clientWriter) send(b []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, err := o.w.Write(b); err != nil {
		return err
	}
	return o.w.Flush()
}

// sendLimit tells the client which limit ended its session.
func sendLimit(o *clientWriter, e *limitErr) {
	_ = o.send(limitMessage(e)) // the session ends right after
}

// limitMessage is the FATAL 54000 of a broken relay limit.
func limitMessage(e *limitErr) []byte {
	b, _ := (&pgproto3.ErrorResponse{Severity: "FATAL", SeverityUnlocalized: "FATAL", Code: "54000", Message: "werbot: " + e.msg}).Encode(nil)
	return b
}
