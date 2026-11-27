package pgconnector

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"strings"

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
	if c.agent.cred == nil {
		return refusedWith(v, "write not approved"), nil
	}
	secret, err := c.agent.cred(c.ctx, v.ApprovalID, sha256.Sum256([]byte(dbproxy.ApprovalText(op))))
	if err != nil {
		return refusedWith(v, "write not approved"), nil
	}
	it := c.allowed(writeKind(frames[0].typ), op, v)
	c.inCycle = false
	hc, err := c.agent.connect(c.ctx, secret)
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
	if err := c.relayWrite(hc.Conn, bufio.NewReaderSize(hc.Conn, ioBuf), it, secret); err != nil {
		return v, err
	}
	c.g.Log(it.line())
	b, _ := (&pgproto3.ReadyForQuery{TxStatus: 'I'}).Encode(nil)
	return v, c.out.send(b)
}

// writeKind is the item kind of an approved write: a query, or the execute of a batch.
func writeKind(first byte) byte {
	if first == 'Q' {
		return 'Q'
	}
	return 'E'
}

// relayWrite forwards the replies of an approved write's connection to the
// client, each whole under the client lock, up to the ReadyForQuery, which
// it keeps back; it counts them to it. A COPY FROM STDIN gets a CopyFail:
// the client's data goes to the agent's own session.
func (c *clientSide) relayWrite(conn io.Writer, r *bufio.Reader, it *item, secret gateway.Secret) error {
	for {
		typ, body, n, err := readReply(r)
		if err != nil {
			return err
		}
		switch typ {
		case 'Z':
			return nil
		case 'G':
			if err := refuseCopy(conn, body, n, r); err != nil {
				return err
			}
			continue
		case 'D':
			it.dataRows++
		case 'C':
			it.tagRows += tagRows(body)
		case 'E':
			var e pgproto3.ErrorResponse
			it.err = secret.Redact(c.log.secret.Redact(errorLine(&e, body)))
		}
		if err := c.forward(typ, body, n, r); err != nil {
			return err
		}
	}
}

// readReply reads the header of a database reply and, for CommandComplete,
// ErrorResponse, ReadyForQuery and CopyInResponse within maxReply, its body;
// body == nil: the n body bytes are still in r.
func readReply(r *bufio.Reader) (typ byte, body []byte, n int, err error) {
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
	if strings.IndexByte("CEZG", typ) < 0 || n > maxReply {
		return typ, nil, n, nil
	}
	body = make([]byte, n)
	_, err = io.ReadFull(r, body)
	return typ, body, n, err
}

// forward writes one reply to the client: body, or its n bytes still in r.
func (c *clientSide) forward(typ byte, body []byte, n int, r io.Reader) error {
	c.out.mu.Lock()
	defer c.out.mu.Unlock()
	var h [5]byte
	h[0] = typ
	binary.BigEndian.PutUint32(h[1:], uint32(n+4)) //nolint:gosec // n came from a 32-bit length
	if _, err := c.out.w.Write(h[:]); err != nil {
		return err
	}
	if body != nil {
		_, err := c.out.w.Write(body)
		return err
	}
	return pass(c.out.w, &c.lr, r, n)
}

// refuseCopy answers a CopyInResponse (body, or its n bytes still in r) of
// an approved write with CopyFail.
func refuseCopy(conn io.Writer, body []byte, n int, r io.Reader) error {
	if err := discard(body, n, r); err != nil {
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
		b = append(b, f.typ)
		b = binary.BigEndian.AppendUint32(b, uint32(len(f.body)+4)) //nolint:gosec // a message read within maxParsed
		b = append(b, f.body...)
	}
	return b
}
