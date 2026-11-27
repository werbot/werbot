package policy

import (
	"os"
	"testing"

	"github.com/cedar-policy/cedar-go"
	"github.com/cedar-policy/cedar-go/x/exp/schema"
	"github.com/cedar-policy/cedar-go/x/exp/schema/resolved"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/werbot/werbot/internal/connector/postgres" // a type with a query action
	"github.com/werbot/werbot/internal/platform/shellclass"
	"github.com/werbot/werbot/internal/platform/sqlclass"
)

// TestMain registers fakeSQL for postgres: the policy tests never load the
// WASM parser (Ruling G13).
func TestMain(m *testing.M) {
	sqlclass.Register(sqlclass.Postgres, fakeSQL{})
	os.Exit(m.Run())
}

// fakeSQL classifies the texts of fakeQueries; any other text has no
// classes, which sqlclass.Classify turns into opaque.
type fakeSQL struct{}

func (fakeSQL) Classify(sql string, _ sqlclass.Options) sqlclass.Result { return fakeQueries[sql] }

var fakeQueries = map[string]sqlclass.Result{
	"SELECT 1":                    {Classes: []shellclass.Class{shellclass.Read}, Statements: 1},
	"SELECT * FROM billing.cards": {Classes: []shellclass.Class{shellclass.Read}, Tables: []string{"billing.cards"}, Statements: 1},
	"DROP TABLE t":                {Classes: []shellclass.Class{shellclass.Destructive}, WriteTables: []string{"public.t"}, Statements: 1},
}

var dbResID = uuid.MustParse("019a0000-0000-7000-8000-0000000000f1")

// dbSnap is agentSnap(rules...) with the postgres resource "pg" (database
// "app") bound to alice too; readonly is its agent_readonly.
func dbSnap(readonly bool, rules ...SnapshotCommandRule) Snapshot {
	s := agentSnap(rules...)
	s.Resources = append(s.Resources, SnapshotResource{ID: dbResID, Name: "pg", Type: "postgres", AgentReadonly: readonly, Database: "app"})
	s.Bindings[0].ResourceIDs = []uuid.UUID{resID, dbResID}
	return s
}

// dbRule is a command rule of alice on pg.
func dbRule(id uuid.UUID, effect, message string, classes ...string) SnapshotCommandRule {
	r := userRule(id, effect, message, classes...)
	r.ResourceIDs = []uuid.UUID{dbResID}
	return r
}

// queryReq is a postgres.query of alice on pg in database app.
func queryReq(classes ...string) Request {
	r := req("10.1.2.3", "DE", "")
	r.ResourceID, r.Action, r.Kind, r.Command, r.Classes, r.DBName = dbResID, "postgres.query", "stream", "sql", classes, "app"
	return r
}

// agentQuery is queryReq made by agent a1 of alice in run 1.
func agentQuery(classes ...string) Request {
	r := asAgent(queryReq(classes...), agent1ID, userID)
	r.Kind = "stream"
	return r
}

// agentConnect is the postgres.connect of agent a1 of alice to pg.
func agentConnect() Request {
	r := agentQuery()
	r.Action, r.Command, r.DBName, r.Port = "postgres.connect", "", "", 5432
	return r
}

func goldenQuerySnap() Snapshot {
	appr := dbRule(apprID, "approval", "", "write")
	appr.Subjects = Subjects{Users: []uuid.UUID{userID}, AllAgents: true}
	allow := dbRule(allowID, "allow", "", "write")
	allow.Subjects = Subjects{AllAgents: true}
	return dbSnap(true, dbRule(denyID, "deny", "No drops", "destructive"), appr, allow)
}

func TestCompileQueryGolden(t *testing.T) {
	_, text, err := Compile(goldenQuerySnap())
	require.NoError(t, err)
	want, err := os.ReadFile("testdata/compile_query.golden.cedar")
	require.NoError(t, err)
	assert.Equal(t, string(want), text)
	assert.Equal(t, text, compileText(t, goldenQuerySnap()), "two compiles give one text")
}

func TestBindingCoversQuery(t *testing.T) {
	for _, c := range []string{"read", "write"} {
		assert.True(t, authorize(t, dbSnap(false), queryReq(c)).Allow, c)
	}
}

func TestQueryCommandRule(t *testing.T) {
	d := authorize(t, dbSnap(false, dbRule(denyID, "deny", "No drops", "destructive")), queryReq("destructive"))
	assert.False(t, d.Allow)
	assert.Equal(t, "No drops", d.Message)
	assert.Equal(t, []string{"command:" + denyID.String()}, d.Reasons)

	s := dbSnap(false, dbRule(apprID, "approval", "", "write"))
	d = authorize(t, s, queryReq("write"))
	assert.False(t, d.Allow)
	assert.Equal(t, []string{ObligationApproval}, d.Obligations)
	granted := queryReq("write")
	granted.ApprovalGranted = true
	assert.True(t, authorize(t, s, granted).Allow)

	// opaque is a class of its own (Ruling G3): a rule must list it.
	assert.True(t, authorize(t, dbSnap(false, dbRule(denyID, "deny", "", "destructive")), queryReq("opaque")).Allow,
		"a destructive rule does not cover opaque")
	d = authorize(t, dbSnap(false, dbRule(denyID, "deny", "", "opaque")), queryReq("opaque"))
	assert.False(t, d.Allow)
	assert.Equal(t, []string{"command:" + denyID.String()}, d.Reasons)
}

func TestAgentDBBaseline(t *testing.T) {
	s := dbSnap(true)
	d := authorize(t, s, agentConnect())
	assert.True(t, d.Allow, "%v", d.Reasons)
	assert.Contains(t, d.Reasons, "agent:baseline:db_connect")
	exec := agentConnect()
	exec.Kind = "exec"
	assert.False(t, authorize(t, s, exec).Allow, "a database connect is a stream")
	d = authorize(t, s, agentQuery("read"))
	assert.True(t, d.Allow, "%v", d.Reasons)
	assert.Contains(t, d.Reasons, "agent:baseline:db_read")
	d = authorize(t, s, agentQuery("write"))
	assert.False(t, d.Allow)
	assert.Empty(t, d.Obligations)
	d = authorize(t, s, agentQuery("destructive"))
	assert.False(t, d.Allow)
	assert.Contains(t, d.Reasons, "agent:baseline:dangerous")
	for _, c := range []string{"opaque", "privilege", "secrets", "egress"} {
		assert.False(t, authorize(t, s, agentQuery(c)).Allow, c)
	}

	appr := dbRule(apprID, "approval", "", "write")
	appr.Subjects = Subjects{AllAgents: true}
	d = authorize(t, dbSnap(true, appr), agentQuery("write"))
	assert.False(t, d.Allow)
	assert.Equal(t, []string{ObligationApproval}, d.Obligations)
	granted := agentQuery("write")
	granted.ApprovalGranted = true
	assert.True(t, authorize(t, dbSnap(true, appr), granted).Allow)

	// Without a verified credential nothing opens the database to agents.
	open := dbSnap(false, appr)
	open.CustomPolicies = []SnapshotCustomPolicy{{ID: customID, Cedar: `permit (principal, action, resource);`}}
	for name, r := range map[string]Request{"connect": agentConnect(), "read": agentQuery("read"), "approved write": granted} {
		d := authorize(t, open, r)
		assert.False(t, d.Allow, name)
		assert.Contains(t, d.Reasons, "agent:baseline:db_readonly", name)
	}
	human := queryReq("write")
	assert.True(t, authorize(t, open, human).Allow, "the sponsor keeps the database")
}

// TestAgentReadonlyMissingAttributeDenied: an entity without agent_readonly
// (Ruling C6) denies agents by the baseline forbid itself, not by an
// evaluation error, even when a custom policy permits everything.
func TestAgentReadonlyMissingAttributeDenied(t *testing.T) {
	s := dbSnap(true)
	s.CustomPolicies = []SnapshotCustomPolicy{{ID: customID, Cedar: `permit (principal, action, resource);`}}
	ps, _, err := Compile(s)
	require.NoError(t, err)
	uid := cedar.NewEntityUID("Werbot::Resource", cedar.String(dbResID.String()))
	e := ps.entities[uid]
	e.Attributes = cedar.NewRecord(cedar.RecordMap{"name": cedar.String("pg"), "type": cedar.String("postgres")})
	ps.entities[uid] = e
	for name, r := range map[string]Request{"connect": agentConnect(), "query": agentQuery("read")} {
		d := ps.Authorize(r)
		assert.False(t, d.Allow, name)
		assert.Contains(t, d.Reasons, "agent:baseline:db_readonly", name)
		assert.NotContains(t, d.Reasons, "error:agent:baseline:db_readonly", name)
	}
}

func TestQueryContextInPolicy(t *testing.T) {
	const orders = `forbid (principal, action, resource) when { context.write_tables.contains("public.orders") };`
	const billing = `forbid (principal, action, resource) when { context.db_name == "billing" || context.tables.contains("billing.cards") };`
	for _, text := range []string{orders, billing} {
		require.NoError(t, ValidateCustom(text), text)
	}
	s := dbSnap(false)
	s.CustomPolicies = []SnapshotCustomPolicy{{ID: customID, Cedar: orders}, {ID: custom2ID, Cedar: billing}}

	write := queryReq("write")
	write.WriteTables = []string{"public.orders"}
	assert.Equal(t, []string{"custom:" + customID.String() + ":0"}, authorize(t, s, write).Reasons)
	write.WriteTables = []string{"public.items"}
	assert.True(t, authorize(t, s, write).Allow, "another table")
	read := queryReq("read")
	read.Tables = []string{"billing.cards"}
	assert.False(t, authorize(t, s, read).Allow)
	read.Tables, read.DBName = nil, "billing"
	assert.False(t, authorize(t, s, read).Allow, "db_name")

	connect := queryReq()
	connect.Action, connect.Command, connect.DBName, connect.Port = "postgres.connect", "", "", 5432
	for name, r := range map[string]Request{"connect": connect, "exec": execReq("read"), "query": queryReq("read")} {
		d := authorize(t, s, r)
		assert.True(t, d.Allow, "%s: %v", name, d.Reasons)
	}
}

func TestSchemaHasQueryContext(t *testing.T) {
	var s schema.Schema
	require.NoError(t, s.UnmarshalCedar([]byte(Schema())))
	res, err := s.Resolve()
	require.NoError(t, err)
	a, ok := res.Actions[cedar.NewEntityUID("Werbot::Action", "postgres.query")]
	require.True(t, ok)
	assert.Equal(t, resolved.StringType{}, a.AppliesTo.Context["db_name"].Type)
	assert.Equal(t, resolved.SetType{Element: resolved.StringType{}}, a.AppliesTo.Context["tables"].Type)
	assert.Equal(t, resolved.SetType{Element: resolved.StringType{}}, a.AppliesTo.Context["write_tables"].Type)
	readonly := res.Entities["Werbot::Resource"].Shape["agent_readonly"]
	assert.Equal(t, resolved.BoolType{}, readonly.Type)
	assert.False(t, readonly.Optional, "every resource entity carries it")
	require.NoError(t, ValidateCustom(`permit (principal, action, resource) when { resource.agent_readonly };`))
}
