package pgconnector

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"net"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/dbproxy"
)

const waitingNotice = "Waiting for approval a1, timeout 5m: https://werbot.test/requests?id=a1"

func TestHoldApprovedHuman(t *testing.T) {
	warm(t)
	cfg, secret, owner := tableDB(t)
	h, rec := newHooks(nil)
	p := serveProxy(t.Context(), t, testResource(t, cfg), secret, withHooks(h))
	var notices []string
	conn := clientOf(t, p, cfg.GetDatabase(), &notices, nil)
	rs, err := conn.Exec(t.Context(), "UPDATE t SET a = 2 WHERE id = 1").ReadAll()
	require.NoError(t, err)
	assert.Equal(t, "UPDATE 1", rs[0].CommandTag.String())
	assert.Equal(t, []string{waitingNotice}, notices)
	assert.Equal(t, "2", value(t, owner, "select a from t"))
	approvals, _ := rec.asked()
	assert.Equal(t, []gateway.ApprovalInput{{Command: "UPDATE t SET a = 2 WHERE id = 1", Classes: []string{"write"}, Reasons: []string{"command:w"}}}, approvals)
	require.NoError(t, conn.Close(t.Context()))
	ls := lines(p.entries(t))
	assert.Equal(t, []logLine{{kind: "query", text: "UPDATE t SET a = 2 WHERE id = 1", rows: 1}}, ls)
}

func TestHoldDenied(t *testing.T) {
	warm(t)
	cfg, secret, owner := tableDB(t)
	h, rec := newHooks(nil)
	p := serveProxy(t.Context(), t, testResource(t, cfg), secret, withHooks(h))
	conn := clientOf(t, p, cfg.GetDatabase(), nil, nil)
	for err, msg := range map[error]string{
		gateway.ErrApprovalDenied:  "approval denied",
		gateway.ErrApprovalTimeout: "approval timed out",
		gateway.ErrNotReady:        "approval unavailable, try again later",
	} {
		rec.approveErr = err
		_, qerr := conn.Exec(t.Context(), "UPDATE t SET a = 2 WHERE id = 1").ReadAll()
		pe := pgErr(t, qerr)
		assert.Equal(t, "42501", pe.Code)
		assert.Equal(t, "werbot: query denied: "+msg, pe.Message)
		assert.Equal(t, "1", value(t, conn, "select a from t"), "the session lives; the row is unchanged")
	}
	assert.Equal(t, "1", value(t, owner, "select a from t"))
}

func TestHoldExtendedBatch(t *testing.T) {
	warm(t)
	cfg, secret, owner := tableDB(t)
	cfg.RecordParams = proto.Bool(false)
	h, rec := newHooks(nil)
	p := serveProxy(t.Context(), t, testResource(t, cfg), secret, withHooks(h))
	conn := clientOf(t, p, cfg.GetDatabase(), nil, nil)
	const text = "UPDATE t SET a = $1 WHERE id = $2"
	res := conn.ExecParams(t.Context(), text, [][]byte{[]byte("3"), []byte("1")}, nil, nil, nil).Read()
	require.NoError(t, res.Err)
	assert.Equal(t, "UPDATE 1", res.CommandTag.String())
	assert.Equal(t, "3", value(t, owner, "select a from t"))
	approvals, _ := rec.asked()
	require.Len(t, approvals, 1, "one approval covers the batch")
	assert.Equal(t, text+"\n-- params: [\"3\",\"1\"]", approvals[0].Command, "the approvers read the parameters without record_params")

	res = conn.ExecParams(t.Context(), text, [][]byte{[]byte("4"), []byte("1")}, nil, nil, nil).Read()
	require.NoError(t, res.Err)
	approvals, _ = rec.asked()
	assert.Len(t, approvals, 2, "other parameters need their own approval")
	require.NoError(t, conn.Close(t.Context()))
	for _, e := range p.entries(t) {
		assert.Nil(t, e.Params, "the query log keeps no parameters without record_params")
	}
}

func TestHoldStillWaitingNotice(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var out bytes.Buffer
		h, rec := newHooks(nil)
		rec.delay = 0
		rec.gate = make(chan struct{})
		go func() { time.Sleep(70 * time.Second); close(rec.gate) }()
		g := dbproxy.NewGuard(&gateway.Session{Recording: &memRecording{}}, h, dbproxy.GuardConfig{Type: "postgres", Dialect: dialect{}})
		defer g.Close()
		c := &clientSide{ctx: t.Context(), g: g, out: &clientWriter{w: bufio.NewWriter(&out)}}
		v, err := c.approve(dbproxy.Op{Kind: "query", Text: "update t set a = 1"},
			dbproxy.Verdict{Obligations: []string{gateway.ObligationApproval}, Classes: []string{"write"}})
		require.NoError(t, err)
		assert.True(t, v.Allow)
		assert.Equal(t, []string{waitingNotice, "Still waiting for approval a1", "Still waiting for approval a1"}, notices(t, out.Bytes()))
	})
}

// notices are the messages of the NoticeResponses in b.
func notices(t *testing.T, b []byte) []string {
	t.Helper()
	var out []string
	for len(b) > 0 {
		n := int(binary.BigEndian.Uint32(b[1:5]))
		var m pgproto3.NoticeResponse
		require.NoError(t, m.Decode(b[5:1+n]))
		out = append(out, m.Message)
		b = b[1+n:]
	}
	return out
}

func TestHoldPipelinedResponsesInOrder(t *testing.T) {
	warm(t)
	cfg, secret, owner := tableDB(t)
	h, rec := newHooks(nil)
	rec.gate = make(chan struct{})
	p := serveProxy(t.Context(), t, testResource(t, cfg), secret, withHooks(h))
	conn, f := rawLogin(t, p, cfg.GetDatabase())
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))
	send(t, f, &pgproto3.Query{String: "select 41 + 1"}, &pgproto3.Query{String: "UPDATE t SET a = 2 WHERE id = 1"})
	var before []string
	for range 5 { // the result of the select and the notice, before the decision
		msg, err := f.Receive()
		require.NoError(t, err)
		before = append(before, traceOf(msg))
	}
	if before[0] == "N "+waitingNotice { // the notice may come before the result
		before = append(before[1:], before[0])
	}
	assert.Equal(t, []string{"T", "D 42", "C SELECT 1", "Z I", "N " + waitingNotice}, before)
	assert.Equal(t, "1", value(t, owner, "select a from t"), "the held statement waits")
	close(rec.gate)
	assert.Equal(t, []string{"C UPDATE 1", "Z I"}, trace(t, f, 1))
	terminate(t, f)
}

func TestHoldNoticeNeverSplitsReply(t *testing.T) {
	warm(t)
	const size = 200 << 10
	f := newFakeDB(t, nil, func(b *pgproto3.Backend, conn net.Conn, _ *pgproto3.StartupMessage) {
		if !fakeLogin(b) {
			return
		}
		fakeLoop(b, func(msg pgproto3.FrontendMessage) {
			q, ok := msg.(*pgproto3.Query)
			if !ok || !strings.HasPrefix(q.String, "select") {
				return
			}
			row, _ := (&pgproto3.DataRow{Values: [][]byte{bytes.Repeat([]byte("x"), size)}}).Encode(nil)
			_, _ = conn.Write(row[:64<<10]) // the gateway relays the first part while the rest is late
			time.Sleep(300 * time.Millisecond)
			_, _ = conn.Write(row[64<<10:])
			b.Send(&pgproto3.CommandComplete{CommandTag: []byte("SELECT 1")})
			b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
			_ = b.Flush()
		})
	})
	cfg := &postgresv1.Config{Host: "127.0.0.1", Port: f.port, Database: "main", Username: "app", Sslmode: "disable"}
	h, rec := newHooks(nil)
	rec.delay = 100 * time.Millisecond // the notice comes while the row is half sent
	rec.approveErr = gateway.ErrApprovalDenied
	p := serveProxy(t.Context(), t, testResource(t, cfg), passwordSecret(marker), withHooks(h))
	conn, fe := rawClient(t, p)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))
	send(t, fe, &pgproto3.Query{String: "select big"}, &pgproto3.Query{String: "UPDATE t SET a = 2 WHERE id = 1"})
	got := trace(t, fe, 2)
	// The notice goes out at a message boundary: after the whole row, before
	// or after the rest of the select's reply.
	n := slices.Index(got, "N "+waitingNotice)
	assert.Greater(t, n, 0, "the notice waits for the end of the row: %v", got)
	if n > 0 {
		got = slices.Delete(got, n, n+1)
	}
	assert.Equal(t, []string{"D " + strconv.Itoa(size) + " bytes", "C SELECT 1", "Z I",
		"E 42501 werbot: query denied: approval denied", "Z I"}, got)
	terminate(t, fe)
}
