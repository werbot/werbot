package pgconnector

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/dbproxy"
)

// stillWaitingEvery is how often a held operation reminds the client; the
// traffic also keeps the session's idle limit and SSH connection alive.
const stillWaitingEvery = 30 * time.Second

// errHeldTooLarge ends a session whose messages held for an approval pass maxParsed.
var errHeldTooLarge = &limitErr{"messages held for an approval exceed 16 MiB"}

// held is an operation of a client message the guard did not allow: a
// deny, or an approval to wait for.
type held struct {
	kind byte
	op   dbproxy.Op
	v    dbproxy.Verdict
}

func (*held) Error() string { return "postgres: operation not allowed" }

// frame is a client message read whole.
type frame struct {
	typ  byte
	body []byte
}

// frame handles one client message; body is nil when its n bytes are still
// in src. After a denied extended message the messages up to Sync are
// dropped, as the database drops them after an error; a Flush still goes,
// so the replies before the denial, and the denial, reach the client.
func (c *clientSide) frame(typ byte, body []byte, n int, src io.Reader) error {
	if c.skipping && typ != 'S' && typ != 'H' && typ != 'X' {
		return discard(body, n, src)
	}
	if typ == 'S' {
		c.skipping = false
	}
	if err := c.cycle(typ); err != nil {
		return err
	}
	if body == nil {
		return c.stream(typ, n, src)
	}
	return c.message(typ, body)
}

// decide answers an operation the guard did not allow: a deny gets its
// error in reply order, an approval obligation holds the operation.
func (c *clientSide) decide(h *held, body []byte) error {
	switch {
	case len(h.v.Obligations) == 0:
		return c.deny(h.kind, h.op, h.v)
	case c.agent != nil:
		return c.holdAgent(h, body)
	}
	return c.hold(h, body)
}

// hold waits for the approval of a person's operation: the replies to the
// messages before it reach the client meanwhile, the messages after it wait.
// A held Parse takes the rest of its batch along, so the approval covers its
// execute with the parameters; once decided, the batch goes on as usual.
func (c *clientSide) hold(h *held, body []byte) error {
	var frames []frame
	op := h.op
	if h.kind == 'P' && !c.replaying {
		var err error
		if frames, err = c.collect(); err != nil {
			return err
		}
		op, _ = batchOp(h.op, frames)
	}
	if err := c.w.Flush(); err != nil {
		return err
	}
	v, err := c.approve(op, h.v)
	switch {
	case err != nil:
		return err
	case v.Allow:
		err = c.send(c.allowed(h.kind, h.op, v), h.kind, body)
	default:
		err = c.deny(h.kind, h.op, v)
	}
	if err != nil {
		return err
	}
	return c.replay(frames)
}

// approve asks for the approval v obliges op to and waits for it, telling
// the client the ticket and reminding it every stillWaitingEvery. An
// unreachable server denies; a session that ends is the error.
func (c *clientSide) approve(op dbproxy.Op, v dbproxy.Verdict) (dbproxy.Verdict, error) {
	stop := make(chan struct{})
	var wg sync.WaitGroup
	defer wg.Wait()
	defer close(stop)
	v, err := c.g.Approve(c.ctx, op, v, func(id, url string, timeout time.Duration) error {
		b, _ := holdNotice(id, url, timeout).Encode(nil)
		if err := c.out.send(b); err != nil {
			return err
		}
		wg.Go(func() { c.stillWaiting(id, stop) })
		return nil
	})
	if errors.Is(err, gateway.ErrNotReady) {
		return refusedWith(v, "approval unavailable, try again later"), nil
	}
	return v, err
}

// holdNotice tells the client which approval its operation waits for.
func holdNotice(id, url string, timeout time.Duration) *pgproto3.NoticeResponse {
	return notice("Waiting for approval " + id + ", timeout " + gateway.FormatWait(timeout) + ": " + url)
}

func notice(msg string) *pgproto3.NoticeResponse {
	return &pgproto3.NoticeResponse{Severity: "NOTICE", SeverityUnlocalized: "NOTICE", Code: "00000", Message: msg}
}

// stillWaiting reminds the client of approval id until stop is closed.
func (c *clientSide) stillWaiting(id string, stop <-chan struct{}) {
	b, _ := notice("Still waiting for approval " + id).Encode(nil)
	t := time.NewTicker(stillWaitingEvery)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			if c.out.send(b) != nil {
				return
			}
		}
	}
}

// deny answers the operation op of a client message of kind with
// ErrorResponse 42501 in reply order and logs it; after an extended message
// the session drops the client's messages up to Sync, as the database does
// after an error.
func (c *clientSide) deny(kind byte, op dbproxy.Op, v dbproxy.Verdict) error {
	msg := "werbot: query denied: " + v.Message
	it := c.newItem(kind, op)
	e := &it.entry
	e.Decision, e.Classes, e.Tables, e.PolicyIDs, e.Error = "deny", v.Classes, v.Tables, v.PolicyIDs, "42501: "+msg
	if c.log.params {
		e.Params = op.Params
	}
	it.kind, it.log, it.ready, it.synth = 'x', true, kind == 'Q' || kind == 'F', errorResponse("42501", msg)
	c.skipping = !it.ready
	return c.answer(it)
}

// answer sends the synthetic reply it in reply order, and logs its line:
// at once when no reply is pending, else the database side does after the
// replies before it.
func (c *clientSide) answer(it *item) error {
	c.out.mu.Lock()
	defer c.out.mu.Unlock()
	if c.q.peek() != nil {
		return c.q.push(it)
	}
	if it.log {
		c.g.Log(it.entry)
	}
	if _, err := c.out.w.Write(it.reply(c.out.status)); err != nil {
		return err
	}
	return c.out.w.Flush()
}

// errorResponse is an ERROR with SQLSTATE code and msg.
func errorResponse(code, msg string) []byte {
	b, _ := (&pgproto3.ErrorResponse{Severity: "ERROR", SeverityUnlocalized: "ERROR", Code: code, Message: msg}).Encode(nil)
	return b
}

// refusedWith is v denied with message.
func refusedWith(v dbproxy.Verdict, message string) dbproxy.Verdict {
	v.Allow, v.Obligations, v.Message = false, nil, message
	return v
}

// collect reads the client's messages after a held Parse whole, up to and
// with a Sync, a Flush (the client then waits for replies) or a Terminate,
// within maxParsed bytes in all.
func (c *clientSide) collect() ([]frame, error) {
	var frames []frame
	total := 0
	for {
		h, err := c.r.Peek(5)
		if err != nil {
			return nil, err
		}
		typ, size := h[0], int(int32(binary.BigEndian.Uint32(h[1:]))) //nolint:gosec // a signed length word
		if size < 4 {
			return nil, errMalformed
		}
		if total += size + 1; total > maxParsed {
			return nil, errHeldTooLarge
		}
		_, _ = c.r.Discard(5)
		body := make([]byte, size-4)
		if _, err := io.ReadFull(c.r, body); err != nil {
			return nil, err
		}
		frames = append(frames, frame{typ, body})
		if typ == 'S' || typ == 'H' || typ == 'X' {
			return frames, nil
		}
	}
}

// replay handles the messages held with a decided batch.
func (c *clientSide) replay(frames []frame) error {
	c.replaying = true
	defer func() { c.replaying = false }()
	for _, f := range frames {
		body, src := f.body, io.Reader(nil)
		if strings.IndexByte("QPBEFC", f.typ) < 0 {
			body, src = nil, bytes.NewReader(f.body)
		}
		if err := c.frame(f.typ, body, len(f.body), src); err != nil {
			return err
		}
	}
	return nil
}

// batchOp is the execute of op's statement in the frames after its Parse,
// with the parameters of its Bind: what an approval of the held batch
// covers. Without one it is op.
func batchOp(op dbproxy.Op, frames []frame) (dbproxy.Op, bool) {
	var b pgproto3.Bind
	var e pgproto3.Execute
	var params []string
	portal, bound := "", false
	for _, f := range frames {
		switch {
		case f.typ == 'B' && b.Decode(f.body) == nil && b.PreparedStatement == op.Statement:
			portal, bound, params = b.DestinationPortal, true, bindParams(&b)
		case f.typ == 'E' && bound && e.Decode(f.body) == nil && e.Portal == portal:
			return dbproxy.Op{Kind: "execute", Text: op.Text, Statement: op.Statement, DB: op.DB, Params: params, Opaque: op.Opaque}, true
		}
	}
	return op, false
}

// discard drops a message: its body, or its n bytes still in src.
func discard(body []byte, n int, src io.Reader) error {
	if body != nil {
		return nil
	}
	_, err := io.CopyN(io.Discard, src, int64(n))
	return err
}
