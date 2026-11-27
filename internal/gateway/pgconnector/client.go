package pgconnector

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"

	"github.com/werbot/werbot/internal/gateway/dbproxy"
)

// clientSide forwards client messages to the database. It reads the
// operations (Query, Parse, Bind, Execute, FunctionCall, Close), checks them
// with the guard and queues every message that gets a reply. A person's
// messages go at once; an agent's next query or batch waits for the reply
// to the previous one.
type clientSide struct {
	ctx     context.Context
	r       *bufio.Reader // the client stream
	w       *bufio.Writer // to the database
	lr      io.LimitedReader
	out     *clientWriter
	q       *queue
	g       *dbproxy.Guard
	log     sessionLog
	agent   *agentWriter // nil for a person
	lex     lexing
	buf     []byte
	hdr     [5]byte
	stmts   map[string]named
	portals map[string]named
	kept    int   // bytes held by stmts and portals, up to maxRetained
	last    *item // the last query or execute: client COPY data counts to it

	skipping  bool // after a denied extended message: drop messages up to Sync
	inCycle   bool // an extended batch waits for its Sync
	relex     bool // an allowed operation may change the lexing settings
	replaying bool // the messages of a decided held batch are being sent

	query pgproto3.Query
	parse pgproto3.Parse
	bind  pgproto3.Bind
	exec  pgproto3.Execute
	call  pgproto3.FunctionCall
	close pgproto3.Close
}

// next handles one client message; the database gets the buffer once the
// client has nothing more pending.
func (c *clientSide) next() error {
	typ, body, n, err := readClient(c.r, c.buf)
	switch {
	case errors.Is(err, errTooLarge):
		_, _ = io.CopyN(io.Discard, c.r, int64(n)) // the client reads the answer once its message is sent
	case err != nil:
		return err
	default:
		err = c.frame(typ, body, n, c.r)
		if body != nil {
			c.buf = body[:0]
		}
		if cap(body) > keepBuf {
			c.buf = nil
		}
	}
	if err != nil {
		if le, ok := errors.AsType[*limitErr](err); ok {
			sendLimit(c.out, le)
		}
		return err
	}
	if c.r.Buffered() == 0 {
		return c.w.Flush()
	}
	return nil
}

// header writes a message header to the database.
func (c *clientSide) header(typ byte, n int) error {
	c.hdr[0] = typ
	binary.BigEndian.PutUint32(c.hdr[1:], uint32(n+4)) //nolint:gosec // n came from a 32-bit length
	_, err := c.w.Write(c.hdr[:])
	return err
}

// message queues and forwards a message whose body was read; an operation
// the guard does not allow is decided first.
func (c *clientSide) message(typ byte, body []byte) error {
	it, err := c.op(typ, body)
	if h, ok := errors.AsType[*held](err); ok {
		return c.decide(h, body)
	}
	if err != nil {
		return err
	}
	return c.send(it, typ, body)
}

// send queues it and forwards its message.
func (c *clientSide) send(it *item, typ byte, body []byte) error {
	if err := c.q.push(it); err != nil {
		return err
	}
	if err := c.header(typ, len(body)); err != nil {
		return err
	}
	_, err := c.w.Write(body)
	return err
}

// stream forwards a message whose n body bytes are still in src.
func (c *clientSide) stream(typ byte, n int, src io.Reader) error {
	switch typ {
	case 'B':
		return c.bigBind(n)
	case 'D', 'S', 'c', 'f': // Describe, Sync, CopyDone, CopyFail
		if err := c.q.push(&item{kind: typ}); err != nil {
			return err
		}
	case 'd': // CopyData
		if c.last != nil {
			c.last.copyIn.Add(int64(n))
		}
	}
	if err := c.header(typ, n); err != nil {
		return err
	}
	if err := pass(c.w, &c.lr, src, n); err != nil {
		return err
	}
	if typ != 'X' {
		return nil
	}
	if err := c.w.Flush(); err != nil {
		return err
	}
	return errTerminate
}

// op is the item of a parsed message.
func (c *clientSide) op(typ byte, body []byte) (*item, error) {
	switch typ {
	case 'Q':
		if c.query.Decode(body) != nil {
			return nil, errMalformed
		}
		return c.checked('Q', dbproxy.Op{Kind: "query", Text: c.query.String, DB: c.log.db})
	case 'P':
		return c.parsed(body)
	case 'B':
		if c.bind.Decode(body) != nil {
			return nil, errMalformed
		}
		return c.bound(c.bind.DestinationPortal, c.bind.PreparedStatement, bindParams(&c.bind))
	case 'E':
		if c.exec.Decode(body) != nil {
			return nil, errMalformed
		}
		p := c.portals[c.exec.Portal]
		return c.checked('E', dbproxy.Op{Kind: "execute", Text: p.text, Statement: p.stmt, DB: c.log.db, Params: p.params})
	case 'F':
		if c.call.Decode(body) != nil {
			return nil, errMalformed
		}
		return c.checked('F', dbproxy.Op{Kind: "query", Text: "fastpath function " + strconv.FormatUint(uint64(c.call.Function), 10), DB: c.log.db})
	}
	return c.closed(body)
}

// parsed keeps the text of a new statement.
func (c *clientSide) parsed(body []byte) (*item, error) {
	if c.parse.Decode(body) != nil {
		return nil, errMalformed
	}
	text := dbproxy.LogText(c.parse.Query)
	if err := c.store(c.stmts, c.parse.Name, named{text: text, size: len(text)}); err != nil {
		return nil, err
	}
	return c.checked('P', dbproxy.Op{Kind: "parse", Text: c.parse.Query, Statement: c.parse.Name, DB: c.log.db})
}

// bound keeps a new portal with its displayed parameters, which an approval
// of its execute covers; a failed Bind is logged as its execute, with the
// parameters when the resource records them.
func (c *clientSide) bound(name, stmt string, params []string) (*item, error) {
	p := named{stmt: stmt, text: c.stmts[stmt].text, params: params}
	for _, v := range params {
		p.size += len(v)
	}
	if err := c.store(c.portals, name, p); err != nil {
		return nil, err
	}
	it := c.newItem('B', dbproxy.Op{Kind: "execute", Text: p.text, Statement: stmt})
	if c.log.params {
		it.entry.Params = params
	}
	return it, nil
}

// closed forgets a closed statement or portal.
func (c *clientSide) closed(body []byte) (*item, error) {
	if c.close.Decode(body) != nil {
		return nil, errMalformed
	}
	switch c.close.ObjectType {
	case 'S':
		c.drop(c.stmts, c.close.Name)
	case 'P':
		c.drop(c.portals, c.close.Name)
	}
	return &item{kind: 'C'}, nil
}

// bigBind streams a Bind over maxParsed: only its portal and statement names
// are read; its parameters display as cut, so no approval covers them.
func (c *clientSide) bigBind(n int) error {
	var names [2]string
	for i := range names {
		b, err := c.r.ReadSlice(0)
		if err != nil {
			return errMalformed
		}
		names[i] = string(b[:len(b)-1])
		n -= len(b)
	}
	if n < 0 {
		return errMalformed
	}
	it, err := c.bound(names[0], names[1], []string{dbproxy.Truncated})
	if err != nil {
		return err
	}
	if err := c.q.push(it); err != nil {
		return err
	}
	if err := c.header('B', n+len(names[0])+len(names[1])+2); err != nil {
		return err
	}
	for _, s := range names {
		_, _ = c.w.WriteString(s)
		_ = c.w.WriteByte(0) // a write error stays in w for pass
	}
	return pass(c.w, &c.lr, c.r, n)
}

// checked asks the guard about op and makes the item of its message; an
// operation it does not allow is a *held. While the session lexes text
// unlike the classifier, or may since an operation of this cycle, op is opaque.
func (c *clientSide) checked(kind byte, op dbproxy.Op) (*item, error) {
	op.Opaque = c.lex.unsafe.Load() || c.relex
	v, err := c.g.Check(c.ctx, op)
	if err != nil {
		return nil, err
	}
	if !v.Allow {
		return nil, &held{kind: kind, op: op, v: v}
	}
	return c.allowed(kind, op, v), nil
}

// allowed is the item of op, which v allows.
func (c *clientSide) allowed(kind byte, op dbproxy.Op, v dbproxy.Verdict) *item {
	it := c.newItem(kind, op)
	it.log = kind != 'P' // a Parse is logged only when it fails
	e := &it.entry
	e.Decision, e.Classes, e.Tables, e.PolicyIDs = "allow", v.Classes, v.Tables, v.PolicyIDs
	if c.log.params {
		e.Params = op.Params
	}
	if kind == 'Q' || kind == 'E' {
		c.last = it
	}
	c.relex = c.relex || (kind != 'P' && relexes(v.Classes))
	return it
}

// newItem is the item of a message with the querylog line of op.
func (c *clientSide) newItem(kind byte, op dbproxy.Op) *item {
	now := time.Now()
	return &item{kind: kind, start: now, entry: dbproxy.Entry{
		T: now.Sub(c.log.t0), Kind: op.Kind, Text: dbproxy.LogText(op.Text), Statement: op.Statement, DB: c.log.db,
	}}
}
