package pgconnector

import (
	"context"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/stretchr/testify/require"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/dbproxy"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/sqlclass"
	"github.com/werbot/werbot/internal/platform/sqlclass/pg"
)

// warm makes the PostgreSQL classifier ready.
func warm(t testing.TB) {
	t.Helper()
	require.NoError(t, pg.Warmup())
}

// testPDP decides with rule and records the classes of every request.
type testPDP struct {
	mu      sync.Mutex
	rule    func(gateway.AccessRequest) gateway.Decision
	classes [][]string
}

func (p *testPDP) Decide(r gateway.AccessRequest) gateway.Decision {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.classes = append(p.classes, r.Classes)
	return p.rule(r)
}

func (*testPDP) Version() string { return "v1" }

func (*testPDP) QueryProfile(gateway.AccessRequest) gateway.QueryProfile {
	return gateway.QueryProfile{Rules: true}
}

func (p *testPDP) decided() [][]string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.classes)
}

var allow = gateway.Decision{Allow: true, Reasons: []string{"binding:b"}}

// writeNeedsApproval allows reads, asks for an approval of writes and denies the rest.
func writeNeedsApproval(r gateway.AccessRequest) gateway.Decision {
	switch {
	case slices.Equal(r.Classes, []string{"read"}), slices.Equal(r.Classes, []string{"write"}) && r.ApprovalGranted:
		return allow
	case slices.Equal(r.Classes, []string{"write"}):
		return gateway.Decision{Reasons: []string{"command:w"}, Obligations: []string{gateway.ObligationApproval}}
	}
	return gateway.Decision{Reasons: []string{"command:d"}}
}

// approvalID and approvalURL are the ticket of every test approval.
const (
	approvalID  = "a1"
	approvalURL = "https://werbot.test/requests?id=a1"
)

// hookRec answers and records what a session asks its hooks. Its settings
// are set before the session starts.
type hookRec struct {
	pdp        *testPDP
	approveErr error          // the end of each approval
	gate       chan struct{}  // when set, an approval waits for it after the notice
	delay      time.Duration  // an approval waits this long before the notice
	cred       gateway.Secret // the write credential
	credErr    error

	mu        sync.Mutex
	approvals []gateway.ApprovalInput
	writes    []writeCall
}

type writeCall struct {
	id  string
	sha [32]byte
}

// newHooks are session hooks on a hookRec with writeNeedsApproval; an
// agent's when limits is set.
func newHooks(limits *gateway.AgentLimits) (gateway.SessionHooks, *hookRec) {
	r := &hookRec{pdp: &testPDP{rule: writeNeedsApproval}}
	return gateway.SessionHooks{PDP: r.pdp, Limits: limits, Approve: r.approve, WriteCredential: r.writeCredential}, r
}

func (r *hookRec) approve(ctx context.Context, in gateway.ApprovalInput, notify func(id, url string, timeout time.Duration) error) error {
	r.mu.Lock()
	r.approvals = append(r.approvals, in)
	r.mu.Unlock()
	time.Sleep(r.delay)
	if err := notify(approvalID, approvalURL, 5*time.Minute); err != nil {
		return err
	}
	if r.gate != nil {
		select {
		case <-r.gate:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return r.approveErr
}

func (r *hookRec) writeCredential(_ context.Context, id string, sha [32]byte) (gateway.Secret, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.writes = append(r.writes, writeCall{id, sha})
	return r.cred, r.credErr
}

func (r *hookRec) asked() ([]gateway.ApprovalInput, []writeCall) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.approvals), slices.Clone(r.writes)
}

func withHooks(h gateway.SessionHooks) func(*proxy) { return func(p *proxy) { p.hooks = h } }

// agentLimits are the default agent limits.
func agentLimits() *gateway.AgentLimits {
	return &gateway.AgentLimits{MaxRows: 1000, MaxResultBytes: 1 << 20, StatementTimeout: 30 * time.Second, MaxQueries: 1000}
}

// dialect classifies every text as res.
type dialect sqlclass.Result

func (d dialect) Classify(string, sqlclass.Options) sqlclass.Result { return sqlclass.Result(d) }

// useDialect classifies the texts of the sessions of t with d.
func useDialect(t *testing.T, d dbproxy.Dialect) {
	old := sqlDialect
	sqlDialect = d
	t.Cleanup(func() { sqlDialect = old })
}

// trace reads up to the n-th ReadyForQuery and names each message with traceOf.
func trace(t testing.TB, f *pgproto3.Frontend, n int) []string {
	t.Helper()
	var out []string
	for n > 0 {
		msg, err := f.Receive()
		require.NoError(t, err)
		if _, ok := msg.(*pgproto3.ReadyForQuery); ok {
			n--
		}
		out = append(out, traceOf(msg))
	}
	return out
}

// traceOf names msg: its type byte and, for errors, notices, tags, rows and
// statuses, what it says.
func traceOf(msg pgproto3.BackendMessage) string {
	switch m := msg.(type) {
	case *pgproto3.ErrorResponse:
		return "E " + m.Code + " " + m.Message
	case *pgproto3.NoticeResponse:
		return "N " + m.Message
	case *pgproto3.CommandComplete:
		return "C " + string(m.CommandTag)
	case *pgproto3.ReadyForQuery:
		return "Z " + string(m.TxStatus)
	case *pgproto3.DataRow:
		if v := m.Values[0]; len(v) > 64 {
			return "D " + strconv.Itoa(len(v)) + " bytes"
		}
		return "D " + string(m.Values[0])
	}
	b, _ := msg.Encode(nil)
	return string(b[:1])
}

// tableDB is a fresh test database with table t (id 1, a 1) and its owner's session.
func tableDB(t *testing.T) (*postgresv1.Config, gateway.Secret, *pgconn.PgConn) {
	t.Helper()
	cfg, secret := realPG(t)
	owner, err := pgconn.Connect(t.Context(), directURL(cfg, secret))
	require.NoError(t, err)
	t.Cleanup(func() { _ = owner.Close(context.Background()) })
	_, err = owner.Exec(t.Context(), "create table t (id int primary key, a int); insert into t values (1, 1)").ReadAll()
	require.NoError(t, err)
	return cfg, secret, owner
}

// agentDB is tableDB with role rw, which may update t (the resource's
// username, its secret the write credential), and role ro, which may only
// read it (agent_username, its secret the agent session's).
func agentDB(t *testing.T) (cfg *postgresv1.Config, ro, rw gateway.Secret, owner *pgconn.PgConn) {
	t.Helper()
	cfg, _, owner = tableDB(t)
	rwName, rwPass := dbtest.Role(t, "")
	roName, roPass := dbtest.Role(t, "")
	_, err := owner.Exec(t.Context(), "grant select, update on t to "+rwName+"; grant select on t to "+roName).ReadAll()
	require.NoError(t, err)
	cfg.Username, cfg.AgentUsername = rwName, roName
	return cfg, passwordSecret(roPass), passwordSecret(rwPass), owner
}

// value is the first value of sql on conn.
func value(t *testing.T, conn *pgconn.PgConn, sql string) string {
	t.Helper()
	rs, err := conn.Exec(t.Context(), sql).ReadAll()
	require.NoError(t, err)
	require.NoError(t, rs[0].Err)
	return string(rs[0].Rows[0][0])
}

// clientOf is a pgconn client of p as an application would open it; notices
// collects the notices it gets.
func clientOf(t *testing.T, p *proxy, db string, notices *[]string, params map[string]string) *pgconn.PgConn {
	t.Helper()
	pc, err := pgconn.ParseConfig(p.connectURL(db))
	require.NoError(t, err)
	pc.OnNotice = func(_ *pgconn.PgConn, n *pgconn.Notice) { // runs on the caller's goroutine
		if notices != nil {
			*notices = append(*notices, n.Message)
		}
	}
	for k, v := range params {
		pc.RuntimeParams[k] = v
	}
	conn, err := pgconn.ConnectConfig(t.Context(), pc)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	return conn
}
