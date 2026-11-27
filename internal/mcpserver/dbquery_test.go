package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"slices"
	"sync"
	"testing"

	gliderssh "github.com/gliderlabs/ssh"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/connectcli"
	"github.com/werbot/werbot/internal/mcpserver/dbquery"
	"github.com/werbot/werbot/internal/platform/sshdgram"
)

// dbGateway is a fake gateway of resource "db": the resource request
// answers postgres on port 5432 and every direct-tcpip stream is a fake
// PostgreSQL; refuse, when set, rejects the stream with its text.
type dbGateway struct {
	*fakeGateway
	refuse string

	mu     sync.Mutex
	logins []string
	opens  []sshdgram.OpenData
	infos  int
}

func newDBGateway(t *testing.T, a *fakeAPI) *dbGateway {
	t.Helper()
	d := &dbGateway{}
	d.fakeGateway = startGateway(t, a.ca.PublicKey(), func(_ *fakeGateway, srv *gliderssh.Server) {
		srv.RequestHandlers = map[string]gliderssh.RequestHandler{sshdgram.ResourceRequest: d.info}
		srv.ChannelHandlers = map[string]gliderssh.ChannelHandler{"direct-tcpip": d.stream}
	})
	a.resources = []*werbotv1.AccessibleResource{{Id: "id-db", Name: "db", Type: "postgres", Kinds: []string{"stream"},
		SshAddress: d.addr, HostKey: d.hostKey, Allowed: true, AllowedClasses: []string{"read"}},
		{Id: "id-srv", Name: "srv", Type: "ssh", Kinds: []string{"exec"}, SshAddress: d.addr, HostKey: d.hostKey, Allowed: true}}
	return d
}

func (d *dbGateway) info(gliderssh.Context, *gliderssh.Server, *gossh.Request) (bool, []byte) {
	d.mu.Lock()
	d.infos++
	d.mu.Unlock()
	b, _ := json.Marshal(sshdgram.ResourceInfo{Type: "postgres", Kinds: []string{"stream"}, Ports: []sshdgram.Port{{Port: 5432}}})
	return true, b
}

func (d *dbGateway) stream(_ *gliderssh.Server, conn *gossh.ServerConn, nc gossh.NewChannel, _ gliderssh.Context) {
	var od sshdgram.OpenData
	_ = gossh.Unmarshal(nc.ExtraData(), &od)
	d.mu.Lock()
	d.logins, d.opens = append(d.logins, conn.User()), append(d.opens, od)
	refuse := d.refuse
	d.mu.Unlock()
	if refuse != "" {
		_ = nc.Reject(gossh.Prohibited, refuse)
		return
	}
	ch, reqs, err := nc.Accept()
	if err != nil {
		return
	}
	go gossh.DiscardRequests(reqs)
	serveFakeDB(ch)
	_ = ch.Close()
}

func (d *dbGateway) refuseWith(text string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.refuse = text
}

func (d *dbGateway) seenStreams() (logins []string, opens []sshdgram.OpenData, infos int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.logins), slices.Clone(d.opens), d.infos
}

// serveFakeDB answers the start-up, then "SELECT 1" with one row, "DROP …"
// with the gateway's denial and anything else with a database error.
func serveFakeDB(rw io.ReadWriter) {
	b := pgproto3.NewBackend(rw, rw)
	if _, err := b.ReceiveStartupMessage(); err != nil {
		return
	}
	b.Send(&pgproto3.AuthenticationOk{})
	b.Send(&pgproto3.BackendKeyData{ProcessID: 1, SecretKey: []byte{0, 0, 0, 1}})
	b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	for b.Flush() == nil {
		msg, err := b.Receive()
		q, ok := msg.(*pgproto3.Query)
		if err != nil || !ok {
			return
		}
		switch q.String {
		case "SELECT 1":
			b.Send(&pgproto3.RowDescription{Fields: []pgproto3.FieldDescription{{Name: []byte("a"), DataTypeOID: 23}}})
			b.Send(&pgproto3.DataRow{Values: [][]byte{[]byte("1")}})
			b.Send(&pgproto3.CommandComplete{CommandTag: []byte("SELECT 1")})
		case "DROP TABLE t":
			b.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "42501", Message: "werbot: query denied: not allowed (destructive)"})
		default:
			b.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "42601", Message: "syntax error"})
		}
		b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	}
}

func dbQuery(t *testing.T, cs *mcp.ClientSession, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	return callTool(t, cs, "db_query", args)
}

func TestDBQueryTool(t *testing.T) {
	a := newFakeAPI(t)
	newDBGateway(t, a)
	cs := startRun(t, Options{Profile: a.profile()}).sdk(t)

	tools, err := cs.ListTools(t.Context(), nil)
	require.NoError(t, err)
	i := slices.IndexFunc(tools.Tools, func(tl *mcp.Tool) bool { return tl.Name == "db_query" })
	require.GreaterOrEqual(t, i, 0)
	ann := tools.Tools[i].Annotations
	require.NotNil(t, ann)
	assert.False(t, ann.ReadOnlyHint)
	require.NotNil(t, ann.DestructiveHint)
	assert.True(t, *ann.DestructiveHint)

	out := decode[dbquery.Result](t, dbQuery(t, cs, map[string]any{"resource": "db", "query": "SELECT 1"}))
	assert.Equal(t, dbquery.Result{Status: "ok", Columns: []string{"a"}, Rows: [][]any{{"1"}}, RowCount: 1}, out)
	out = decode[dbquery.Result](t, dbQuery(t, cs, map[string]any{"resource": "db", "query": "DROP TABLE t"}))
	assert.Equal(t, dbquery.Result{Status: "denied", Message: "werbot: query denied: not allowed (destructive)"}, out)
	assert.Contains(t, errText(t, dbQuery(t, cs, map[string]any{"resource": "db", "query": "SELEC 1"})), "syntax error",
		"a database error is a tool error")

	for args, text := range map[string]string{
		`{"resource":"srv","query":"SELECT 1"}`:                  "resource does not accept queries",
		`{"resource":"nope","query":"SELECT 1"}`:                 "resource nope not found",
		`{"resource":"","query":"SELECT 1"}`:                     "resource is required",
		`{"resource":"db","query":""}`:                           "query is required",
		`{"resource":"db","query":"SELECT 1","max_rows":100001}`: "max_rows must be between 0 and 100000",
		`{"resource":"db","query":"SELECT 1","max_rows":-1}`:     "max_rows must be between 0 and 100000",
	} {
		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(args), &m))
		assert.Equal(t, text, errText(t, dbQuery(t, cs, m)), args)
	}

	calls := state(a, func() []*werbotv1.RecordToolCallRequest { return slices.Clone(a.toolCalls) })
	require.Len(t, calls, 9)
	assert.Equal(t, "db_query", calls[0].Tool)
	assert.Equal(t, map[string]any{"resource": "db", "query": "SELECT 1"}, calls[0].Arguments.AsMap(), "the arguments are recorded masked by key")
	statuses := []string{}
	for _, f := range finishes(a) {
		statuses = append(statuses, f.status)
	}
	assert.Equal(t, []string{"ok", "denied", "error", "error", "error", "error", "error", "error", "error"}, statuses)
}

// db_query dials the resource's gateway as <login>+<resource> with the run
// certificate, asks for the resource and opens one warm direct-tcpip stream.
func TestDBQueryUsesAgentStream(t *testing.T) {
	a := newFakeAPI(t)
	g := newDBGateway(t, a)
	cs := startRun(t, Options{Profile: a.profile()}).sdk(t)

	for range 2 {
		assert.Equal(t, "ok", decode[dbquery.Result](t, dbQuery(t, cs, map[string]any{"resource": "db", "query": "SELECT 1"})).Status)
	}
	logins, opens, infos := g.seenStreams()
	assert.Equal(t, []string{"alice+db"}, logins, "one warm stream for both calls")
	assert.Equal(t, []sshdgram.OpenData{{Host: "db", Port: 5432, OriginAddr: "127.0.0.1"}}, opens)
	assert.Equal(t, 1, infos)
}

// A stream the gateway refuses is a denial when the gateway denied access,
// else a tool error with the gateway's text.
func TestDBQueryOpenRefused(t *testing.T) {
	a := newFakeAPI(t)
	g := newDBGateway(t, a)
	cs := startRun(t, Options{Profile: a.profile()}).sdk(t)

	g.refuseWith("Access denied")
	out := decode[dbquery.Result](t, dbQuery(t, cs, map[string]any{"resource": "db", "query": "SELECT 1"}))
	assert.Equal(t, dbquery.Result{Status: "denied", Message: "Access denied"}, out)
	g.refuseWith("Agent limit exceeded")
	assert.Equal(t, "Agent limit exceeded", errText(t, dbQuery(t, cs, map[string]any{"resource": "db", "query": "SELECT 1"})))
	assert.Equal(t, []string{"denied", "error"}, []string{finishes(a)[0].status, finishes(a)[1].status})
}

// stdin EOF closes the warm streams before the run ends.
func TestDBQueryPoolClosedOnEOF(t *testing.T) {
	a := newFakeAPI(t)
	newDBGateway(t, a)
	var mu sync.Mutex
	var closedBeforeEnd []bool
	dial := func(ctx context.Context, ep connectcli.Endpoint) (net.Conn, error) {
		conn, err := connectcli.Dial(ctx, ep)
		if err != nil {
			return nil, err
		}
		return onClose{conn, func() {
			mu.Lock()
			closedBeforeEnd = append(closedBeforeEnd, len(a.endedRuns()) == 0)
			mu.Unlock()
		}}, nil
	}
	h := startRun(t, Options{Profile: a.profile(), Dial: dial})
	cs := h.sdk(t)
	assert.Equal(t, "ok", decode[dbquery.Result](t, dbQuery(t, cs, map[string]any{"resource": "db", "query": "SELECT 1"})).Status)

	require.NoError(t, h.stop(t))
	assert.Equal(t, []string{a.runID.String()}, a.endedRuns())
	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, closedBeforeEnd, "the warm stream was closed")
	assert.True(t, closedBeforeEnd[0], "the warm stream closed before EndRun")
}

// onClose is a net.Conn that calls f on every Close.
type onClose struct {
	net.Conn
	f func()
}

func (c onClose) Close() error {
	c.f()
	return c.Conn.Close()
}
