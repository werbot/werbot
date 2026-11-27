package pgconnector

import (
	"bufio"
	"encoding/binary"
	"io"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"

	"github.com/werbot/werbot/internal/gateway/dbproxy"
)

// dbSide forwards database replies to the client. It reads only headers, and
// the bodies of CommandComplete, ErrorResponse, ReadyForQuery and
// ParameterStatus; DataRow, CopyData and the rest pass through. Replies close
// the queued items; a finished operation becomes its querylog line; a
// synthetic reply queued behind them goes to the client in their place.
type dbSide struct {
	r      *bufio.Reader // the database connection
	w      *bufio.Writer // to the client: out.w, written under out.mu
	out    *clientWriter
	lex    *lexing
	lr     io.LimitedReader
	q      *queue
	g      *dbproxy.Guard
	sess   sessionLog
	lim    *limiter // nil: rows stream through untouched
	cur    *item    // the head of q once looked up
	copyIn bool     // cur runs a COPY FROM STDIN
	skip   bool     // an extended message failed: the database skips to the next Sync
	buf    []byte
	er     pgproto3.ErrorResponse
}

// next forwards one reply whole under the client lock; the client gets the
// buffer once the database has nothing more pending.
func (d *dbSide) next() error {
	h, err := d.r.Peek(5)
	if err != nil {
		return err
	}
	typ, size := h[0], int(int32(binary.BigEndian.Uint32(h[1:]))) //nolint:gosec // a signed length word
	if size < 4 {
		return errMalformed
	}
	d.out.mu.Lock()
	defer d.out.mu.Unlock()
	if d.lim != nil {
		err = d.limited(h, typ, size-4)
	} else {
		err = d.forward(h, typ, size-4)
	}
	if err != nil {
		return err
	}
	if d.r.Buffered() == 0 {
		return d.w.Flush()
	}
	return nil
}

// forward writes the peeked header h of a reply and forwards its n body bytes.
func (d *dbSide) forward(h []byte, typ byte, n int) error {
	if _, err := d.w.Write(h); err != nil {
		return err
	}
	_, _ = d.r.Discard(5)
	return d.message(typ, n)
}

// header writes the header of a reply of n body bytes.
func (d *dbSide) header(typ byte, n int) error {
	var h [5]byte
	h[0] = typ
	binary.BigEndian.PutUint32(h[1:], uint32(n+4)) //nolint:gosec // n came from a 32-bit length
	_, err := d.w.Write(h[:])
	return err
}

// message forwards the body of a reply and applies it to the queue.
func (d *dbSide) message(typ byte, n int) error {
	switch typ {
	case 'D', 'd': // DataRow, CopyData
		d.count(typ, n)
	case 'C', 'E', 'Z', 'S':
		if n <= maxReply {
			return d.parsed(typ, n)
		}
	case 'G': // CopyInResponse
		d.copyIn = true
	}
	if err := pass(d.w, &d.lr, d.r, n); err != nil {
		return err
	}
	d.reply(typ, nil)
	return nil
}

// count adds a DataRow or CopyData of n bytes to the head's querylog line.
func (d *dbSide) count(typ byte, n int) {
	if it := d.head(); it != nil {
		it.bytes += int64(n)
		if typ == 'D' {
			it.dataRows++
		}
	}
}

// parsed forwards a reply read whole and applies it.
func (d *dbSide) parsed(typ byte, n int) error {
	body, err := d.read(n)
	if err != nil {
		return err
	}
	if _, err := d.w.Write(body); err != nil {
		return err
	}
	if typ == 'S' {
		d.lex.status(body)
	}
	d.reply(typ, body)
	return nil
}

// read reads the next n body bytes into d.buf, which a large body does not
// keep past its message.
func (d *dbSide) read(n int) ([]byte, error) {
	if cap(d.buf) < n {
		d.buf = make([]byte, n)
	}
	body := d.buf[:n]
	if cap(d.buf) > keepBuf {
		d.buf = nil
	}
	_, err := io.ReadFull(d.r, body)
	return body, err
}

// pops maps a reply to the queued message it completes without a line:
// ParseComplete, BindComplete, CloseComplete, and the end of a Describe.
var pops = [256]byte{'1': 'P', '2': 'B', '3': 'C', 'T': 'D', 'n': 'D'}

// reply applies a reply to the head of the queue; body is nil when unread.
func (d *dbSide) reply(typ byte, body []byte) {
	it := d.head()
	switch {
	case it == nil:
	case pops[typ] != 0:
		if it.kind == pops[typ] {
			d.pop()
		}
	case typ == 'C':
		d.endCopy()
		d.complete(it, body)
	case typ == 'I' || typ == 's': // EmptyQueryResponse, PortalSuspended
		if it.kind == 'E' {
			d.finish(it)
		}
	case typ == 'E':
		d.failed(it, body)
	case typ == 'Z':
		d.ready(body)
	}
}

// complete applies a CommandComplete: it ends an Execute; a simple query
// adds the rows of each of its statements.
func (d *dbSide) complete(it *item, body []byte) {
	switch it.kind {
	case 'E':
		it.tagRows = tagRows(body)
		d.finish(it)
	case 'Q':
		it.tagRows += tagRows(body)
	}
}

// failed applies an ErrorResponse: a query keeps the first error until
// ReadyForQuery; a failed Sync (a deferred check at the implicit commit) is
// an execute line of its own; a failed Parse, Bind or Execute is logged now,
// and the database skips every message up to the next Sync.
func (d *dbSide) failed(it *item, body []byte) {
	text := d.errorText(body)
	d.endCopy()
	switch it.kind {
	case 'Q', 'F':
		if it.err == "" {
			it.err = text
		}
	case 'S':
		d.g.Log(dbproxy.Entry{T: time.Since(d.sess.t0), Kind: "execute", DB: d.sess.db, Error: text})
	default: // Parse, Bind, Execute, Describe, Close
		d.skip = true
		it.err, it.log = text, it.kind != 'D' && it.kind != 'C'
		d.finish(it)
	}
}

// endCopy ends the COPY FROM STDIN of the head: the Syncs the database read
// in it get no reply.
func (d *dbSide) endCopy() {
	if d.copyIn {
		d.copyIn = false
		d.q.copied()
	}
}

// ready applies a ReadyForQuery with body: it ends the cycle up to the Sync,
// query or function call it answers. After an extended-protocol error it
// answers the Sync: the messages before it, queries too, never ran and get
// no line, and a denial among them is not sent.
func (d *dbSide) ready(body []byte) {
	if len(body) == 1 {
		d.out.status = body[0]
	}
	defer func() { d.skip = false }()
	for it := d.head(); it != nil; it = d.head() {
		query := (it.kind == 'Q' || it.kind == 'F') && !d.skip
		if query {
			d.log(it) // before pop sends what waits behind it
		}
		d.pop()
		if query || it.kind == 'S' {
			return
		}
	}
}

func (d *dbSide) head() *item {
	if d.cur == nil {
		d.cur = d.q.peek()
	}
	return d.cur
}

// pop drops the head, then sends the synthetic replies now at the head and
// logs their lines; the client never sees a denial within an extended batch
// the database skips.
func (d *dbSide) pop() {
	d.q.pop()
	d.cur = nil
	for it := d.q.peek(); it != nil && it.kind == 'x'; it = d.q.peek() {
		d.q.pop()
		if it.log {
			d.g.Log(it.entry)
		}
		if it.ready || !d.skip {
			_, _ = d.w.Write(it.reply(d.out.status)) // an error stays in w for the flush
		}
	}
}

// finish logs it when its end is a line and pops it.
func (d *dbSide) finish(it *item) {
	if it.log {
		d.log(it)
	}
	d.pop()
}

func (d *dbSide) log(it *item) { d.g.Log(it.line()) }

// errorText is the logged text of an ErrorResponse without the session credential.
func (d *dbSide) errorText(body []byte) string {
	return d.sess.secret.Redact(errorLine(&d.er, body))
}

// errorLine is the logged text of an ErrorResponse body decoded with e:
// SQLSTATE and message.
func errorLine(e *pgproto3.ErrorResponse, body []byte) string {
	if body == nil || e.Decode(body) != nil {
		return "unreadable error"
	}
	return e.Code + ": " + e.Message
}
