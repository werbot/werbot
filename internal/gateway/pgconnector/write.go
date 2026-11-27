package pgconnector

import (
	"bufio"
	"context"
	"crypto/sha256"
	"io"
	"sync/atomic"

	"github.com/jackc/pgx/v5/pgproto3"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/dbproxy"
)

// runApprovedWrite runs the frames of the approved op on a connection of
// its own with the primary credential the approval releases for exactly
// that text, and answers the client as that database does but for the last
// ReadyForQuery: 'I', the status of the agent's own session. A refused
// credential is a refused verdict.
func (c *clientSide) runApprovedWrite(frames []frame, op dbproxy.Op, v dbproxy.Verdict) (dbproxy.Verdict, error) {
	cred := c.agent.h.WriteCredential
	if cred == nil {
		return dbproxy.Refused(v, "write not approved"), nil
	}
	secret, err := cred(c.ctx, v.ApprovalID, sha256.Sum256([]byte(dbproxy.ApprovalText(op))))
	if err != nil {
		return dbproxy.Refused(v, "write not approved"), nil
	}
	it := c.allowed('Q', op, v) // every reply of the write counts to it, as to a query
	c.inCycle = false
	hc, canceler, err := c.agent.connect(c.ctx, secret)
	if err != nil {
		it.err = "08006: approved write: connection failed"
		c.g.Log(it.line())
		return v, c.answer(&item{kind: 'x', ready: true, synth: errorResponse("08006", "werbot: approved write: connection to "+c.agent.name+" failed")})
	}
	defer hc.Conn.Close()
	defer context.AfterFunc(c.ctx, func() { _ = hc.Conn.Close() })()
	if _, err := hc.Conn.Write(encodeFrames(frames)); err != nil {
		return v, err
	}
	cancel := func() {
		ctx, stop := context.WithTimeout(c.ctx, cancelWait)
		defer stop()
		_ = canceler.CancelRequest(ctx)
	}
	if err := c.relayWrite(hc.Conn, bufio.NewReaderSize(hc.Conn, ioBuf), it, secret, cancel); err != nil {
		return v, err
	}
	it.err = c.log.secret.Redact(it.err)
	c.g.Log(it.line())
	b, _ := (&pgproto3.ReadyForQuery{TxStatus: 'I'}).Encode(nil)
	return v, c.out.send(b)
}

// relayWrite forwards the replies of an approved write's connection r to
// the client as the session's own replies go (whole under the client lock,
// within the agent's limits, cut with cancel, masked), up to the
// ReadyForQuery, which it keeps back; they count to it. A COPY FROM STDIN
// gets a CopyFail on conn: the client's data goes to the agent's own session.
func (c *clientSide) relayWrite(conn io.Writer, r *bufio.Reader, it *item, secret gateway.Secret, cancel func()) error {
	lim := newLimiter(c.agent.h, cancel)
	lim.write = true
	d := &dbSide{r: r, w: c.out.w, out: c.out, lex: &lexing{}, resets: new(atomic.Uint64), q: &queue{items: []*item{it}},
		g: c.g, sess: sessionLog{t0: c.log.t0, db: c.log.db, secret: secret}, lim: lim}
	for {
		typ, n, err := readHeader(r)
		switch {
		case err != nil:
			return err
		case typ == 'Z':
			_, err = r.Discard(n)
			return err
		case typ == 'G':
			err = refuseCopy(conn, n, r)
		default:
			err = d.relay(typ, n)
		}
		if err != nil {
			return err
		}
	}
}

// refuseCopy answers a CopyInResponse (its n body bytes still in r) of an
// approved write with CopyFail.
func refuseCopy(conn io.Writer, n int, r *bufio.Reader) error {
	if _, err := r.Discard(n); err != nil {
		return err
	}
	b, _ := (&pgproto3.CopyFail{Message: "werbot: approved writes take no COPY data"}).Encode(nil)
	_, err := conn.Write(b)
	return err
}

// encodeFrames is the wire form of frames.
func encodeFrames(frames []frame) []byte {
	var b []byte
	for _, f := range frames {
		b = append(appendHeader(b, f.typ, len(f.body)), f.body...)
	}
	return b
}
