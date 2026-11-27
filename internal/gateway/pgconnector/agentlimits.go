package pgconnector

import (
	"context"
	"encoding/binary"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgproto3"

	"github.com/werbot/werbot/internal/gateway"
)

// cancelWait bounds the CancelRequest the gateway sends at a cut.
const cancelWait = 5 * time.Second

// limiter bounds what an agent's cycle reads (agent.limits) and masks the
// cells of the rows of a masked session. A DataRow or CopyData past a
// limit cuts the reply: it and every later row of the cycle are dropped, the
// database query is canceled at once, and the end of the reply tells the
// client. Only the database side uses it.
type limiter struct {
	bounded           bool  // an agent's: rows and bytes are limited
	maxRows, maxBytes int64 // zero limits let no row through
	mask              func([]byte) []byte
	cancel            func() // cancels the database's running query, synchronously

	rows, bytes int64 // forwarded in the current cycle; bytes after masking
	cut         bool  // rows are dropped up to the end of the reply
	byBytes     bool  // the cut came from maxBytes
	copyOut     bool  // the reply is a COPY TO STDOUT
	dr          pgproto3.DataRow
	out         []byte
}

// newLimiter is the limiter of a session of h; nil for a person whose
// output is not masked: the replies stream through untouched.
func newLimiter(h gateway.SessionHooks, cancel func()) *limiter {
	if h.Limits == nil && h.Mask == nil {
		return nil
	}
	l := &limiter{mask: h.Mask, cancel: cancel}
	if h.Limits != nil {
		l.bounded, l.maxRows, l.maxBytes = true, int64(h.Limits.MaxRows), h.Limits.MaxResultBytes
	}
	return l
}

// cutCanceler cancels the running query of the session registered under
// key for resource id, as a client's cancel request does, within cancelWait.
func cutCanceler(ctx context.Context, key uint64, id uuid.UUID) func() {
	return func() {
		ctx, stop := context.WithTimeout(ctx, cancelWait)
		defer stop()
		cancels.Cancel(ctx, key, id)
	}
}

// fits reports whether a row of n bytes stays within the limits.
func (l *limiter) fits(n int) bool {
	return !l.bounded || (l.rows < l.maxRows && l.bytes+int64(n) <= l.maxBytes)
}

// startCut cuts the reply and cancels its query, once.
func (l *limiter) startCut() {
	if l.cut {
		return
	}
	l.cut, l.byBytes = true, l.rows < l.maxRows
	l.cancel()
}

// masked is the message of a DataRow or CopyData body with its cells masked.
func (l *limiter) masked(typ byte, body []byte) ([]byte, error) {
	if typ == 'd' {
		return (&pgproto3.CopyData{Data: l.mask(body)}).Encode(l.out[:0])
	}
	if l.dr.Decode(body) != nil {
		return nil, errMalformed
	}
	for i, v := range l.dr.Values {
		if v != nil {
			l.dr.Values[i] = l.mask(v)
		}
	}
	return l.dr.Encode(l.out[:0])
}

// truncNotice tells the client its result was cut at limit rows or bytes.
func truncNotice(byBytes bool, limit int64) []byte {
	unit := " rows"
	if byBytes {
		unit = " bytes"
	}
	b, _ := (&pgproto3.NoticeResponse{Severity: "WARNING", SeverityUnlocalized: "WARNING", Code: "01000",
		Message: "werbot: result truncated at " + strconv.FormatInt(limit, 10) + unit}).Encode(nil)
	return b
}

// limited handles a reply of a session with a limiter; h is its header.
func (d *dbSide) limited(h []byte, typ byte, n int) error {
	l := d.lim
	switch {
	case typ == 'D' || typ == 'd':
		_, _ = d.r.Discard(5)
		return d.row(typ, n)
	case l.cut && strings.IndexByte("CEs", typ) >= 0 && n <= maxReply:
		_, _ = d.r.Discard(5)
		return d.cutEnd(typ, n)
	case typ == 'Z':
		l.rows, l.bytes, l.cut, l.copyOut = 0, 0, false, false
	}
	return d.forward(h, typ, n)
}

// row forwards a DataRow or CopyData of n body bytes, masked when the
// session masks; a row past a limit cuts the reply. The querylog counts
// the bytes the database sent.
func (d *dbSide) row(typ byte, n int) error {
	l := d.lim
	l.copyOut = l.copyOut || typ == 'd'
	if l.cut || !l.fits(n) {
		l.startCut()
		_, err := d.r.Discard(n)
		return err
	}
	if l.mask == nil {
		d.counted(typ, n, n)
		if err := d.header(typ, n); err != nil {
			return err
		}
		return pass(d.w, &d.lr, d.r, n)
	}
	return d.maskedRow(typ, n)
}

// maskedRow forwards a DataRow or CopyData of n body bytes with its cells
// masked; one that masking made longer than the limits cuts the reply.
func (d *dbSide) maskedRow(typ byte, n int) error {
	l := d.lim
	if !l.bounded && n > maxParsed {
		_, _ = d.w.Write(limitMessage(errTooLarge)) // a person's row too large to mask ends the session
		return errTooLarge
	}
	body, err := d.read(n)
	if err != nil {
		return err
	}
	if l.out, err = l.masked(typ, body); err != nil {
		return err
	}
	if !l.fits(len(l.out) - 5) {
		l.startCut()
		return nil
	}
	d.counted(typ, n, len(l.out)-5)
	_, err = d.w.Write(l.out)
	if cap(l.out) > keepBuf {
		l.out = nil
	}
	return err
}

// counted adds a forwarded row: n bytes from the database to the querylog
// line, sent bytes to the limits.
func (d *dbSide) counted(typ byte, n, sent int) {
	d.lim.rows++
	d.lim.bytes += int64(sent)
	d.count(typ, n)
}

// cutEnd ends a cut reply with the CommandComplete, ErrorResponse or
// PortalSuspended of n body bytes: the client reads the truncation notice,
// then a CommandComplete with the rows it got in place of the end or of the
// cancel's error; any other error, and the cancel's error of a COPY, passes.
// The queue sees the database's reply.
func (d *dbSide) cutEnd(typ byte, n int) error {
	body, err := d.read(n)
	if err != nil {
		return err
	}
	l := d.lim
	l.cut = false
	limit := l.maxRows
	if l.byBytes {
		limit = l.maxBytes
	}
	out := truncNotice(l.byBytes, limit)
	if typ == 'C' || (typ == 'E' && !l.copyOut && errorCode(&d.er, body) == "57014") {
		verb := "SELECT "
		if l.copyOut {
			verb = "COPY "
		}
		tag := verb + strconv.FormatInt(l.rows, 10)
		out, _ = (&pgproto3.CommandComplete{CommandTag: []byte(tag)}).Encode(out)
		if typ == 'C' {
			body = append([]byte(tag), 0)
		}
	} else {
		out = binary.BigEndian.AppendUint32(append(out, typ), uint32(n+4)) //nolint:gosec // n ≤ maxReply
		out = append(out, body...)
	}
	if _, err := d.w.Write(out); err != nil {
		return err
	}
	d.reply(typ, body)
	return nil
}

// errorCode is the SQLSTATE of an ErrorResponse body; "" when unreadable.
func errorCode(e *pgproto3.ErrorResponse, body []byte) string {
	if e.Decode(body) != nil {
		return ""
	}
	return e.Code
}
