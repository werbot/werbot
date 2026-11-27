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
// the bodies of CommandComplete, ErrorResponse and ReadyForQuery; DataRow,
// CopyData and the rest pass through. Replies close the queued items; a
// finished operation becomes its querylog line.
type dbSide struct {
	r      *bufio.Reader // the database connection
	w      *bufio.Writer // to the client
	lr     io.LimitedReader
	q      *queue
	g      *dbproxy.Guard
	sess   sessionLog
	cur    *item // the head of q once looked up
	copyIn bool  // cur runs a COPY FROM STDIN
	skip   bool  // an extended message failed: the database skips to the next Sync
	buf    []byte
	er     pgproto3.ErrorResponse
}

// next forwards one reply; the client gets the buffer once the database has
// nothing more pending.
func (d *dbSide) next() error {
	h, err := d.r.Peek(5)
	if err != nil {
		return err
	}
	typ, size := h[0], int(int32(binary.BigEndian.Uint32(h[1:]))) //nolint:gosec // a signed length word
	if size < 4 {
		return errMalformed
	}
	if _, err := d.w.Write(h); err != nil {
		return err
	}
	_, _ = d.r.Discard(5)
	if err := d.message(typ, size-4); err != nil {
		return err
	}
	if d.r.Buffered() == 0 {
		return d.w.Flush()
	}
	return nil
}

// message forwards the body of a reply and applies it to the queue.
func (d *dbSide) message(typ byte, n int) error {
	switch typ {
	case 'D', 'd': // DataRow, CopyData
		if it := d.head(); it != nil {
			it.bytes += int64(n)
			if typ == 'D' {
				it.dataRows++
			}
		}
	case 'C', 'E', 'Z':
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

// parsed forwards a reply read whole and applies it.
func (d *dbSide) parsed(typ byte, n int) error {
	if cap(d.buf) < n {
		d.buf = make([]byte, n)
	}
	body := d.buf[:n]
	if _, err := io.ReadFull(d.r, body); err != nil {
		return err
	}
	if _, err := d.w.Write(body); err != nil {
		return err
	}
	d.reply(typ, body)
	if cap(d.buf) > keepBuf {
		d.buf = nil
	}
	return nil
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
		d.ready()
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

// ready applies a ReadyForQuery: it ends the cycle up to the Sync, query or
// function call it answers. After an extended-protocol error it answers the
// Sync: the messages before it, queries too, never ran and get no line.
func (d *dbSide) ready() {
	skipped := d.skip
	d.skip = false
	for it := d.head(); it != nil; it = d.head() {
		d.pop()
		switch {
		case it.kind == 'S':
			return
		case (it.kind == 'Q' || it.kind == 'F') && !skipped:
			d.log(it)
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

func (d *dbSide) pop() {
	d.q.pop()
	d.cur = nil
}

// finish pops it and logs it when its end is a line.
func (d *dbSide) finish(it *item) {
	d.pop()
	if it.log {
		d.log(it)
	}
}

func (d *dbSide) log(it *item) {
	e := it.entry
	e.Rows = max(it.dataRows, it.tagRows)
	e.Bytes = it.bytes + it.copyIn.Load()
	e.DurationUS = time.Since(it.start).Microseconds()
	e.Error = it.err
	d.g.Log(e)
}

// errorText is the logged text of an ErrorResponse: SQLSTATE and message.
func (d *dbSide) errorText(body []byte) string {
	if body == nil || d.er.Decode(body) != nil {
		return "unreadable error"
	}
	return d.sess.secret.Redact(d.er.Code + ": " + d.er.Message)
}
