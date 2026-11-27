package app

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/clientauth"
	"github.com/werbot/werbot/internal/mcpserver/dbquery"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// pgTarget is the database of a postgres resource: a fresh database of the
// test cluster owned by the primary role, with table orders (3 rows), and the
// login role cfg.AgentUsername that may only read orders.
type pgTarget struct {
	pool     *pgxpool.Pool // the primary role
	cfg      *postgresv1.Config
	password string // of the primary role
	roPass   string // of the agent role
}

// newPGTarget creates the target; grants run as the primary role after it
// granted SELECT on orders to the agent role (%s is the agent role).
func newPGTarget(t *testing.T, grants ...string) pgTarget {
	t.Helper()
	pool, url := dbtest.New(t)
	pc, err := pgconn.ParseConfig(url)
	require.NoError(t, err)
	ro, roPass := dbtest.Role(t, "")
	sql := []string{"CREATE TABLE orders (id int PRIMARY KEY, note text)",
		"INSERT INTO orders VALUES (1, 'a'), (2, 'b'), (3, 'c')", "GRANT SELECT ON orders TO " + ro}
	for _, g := range append(sql, grants...) {
		_, err := pool.Exec(context.Background(), strings.ReplaceAll(g, "%s", ro))
		require.NoError(t, err, g)
	}
	return pgTarget{pool: pool, password: pc.Password, roPass: roPass, cfg: &postgresv1.Config{Host: pc.Host, Port: uint32(pc.Port),
		Database: pc.Database, Username: pc.User, Sslmode: "disable", AgentUsername: ro, AllowedDatabases: []string{pc.Database}}}
}

// note is the note of order id.
func (tg pgTarget) note(t *testing.T, id int) string {
	t.Helper()
	var n string
	require.NoError(t, tg.pool.QueryRow(context.Background(), "SELECT note FROM orders WHERE id = $1", id).Scan(&n))
	return n
}

// addDB creates the postgres resource "db" (label env=prod) on tg in the pool
// "gw" with the primary and the agent_readonly password, bound to alice.
func (s *stack) addDB(t *testing.T, tg pgTarget) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	cfg, err := anypb.New(tg.cfg)
	require.NoError(t, err)
	res, err := s.api.res.CreateResource(ctx, connect.NewRequest(&werbotv1.CreateResourceRequest{
		Name: "db", Type: "postgres", GatewayId: s.gatewayID.String(), Config: cfg, Labels: map[string]string{"env": "prod"}}))
	require.NoError(t, err)
	id := res.Msg.GetResource().GetId()
	for slot, secret := range map[string]string{"": tg.password, "agent_readonly": tg.roPass} {
		_, err = s.api.res.SetCredential(ctx, connect.NewRequest(&werbotv1.SetCredentialRequest{
			ResourceId: id, Kind: "password", Value: []byte(secret), Slot: slot}))
		require.NoError(t, err)
	}
	_, err = s.api.acc.CreateBinding(ctx, connect.NewRequest(&werbotv1.CreateBindingRequest{
		SubjectType: "user", SubjectId: s.alice, ResourceId: id}))
	require.NoError(t, err)
	return uuid.MustParse(id)
}

// dbAgentStack is an agent stack with the resource "db" on a new target; a
// second instance of the pool joins after it, so its first health pass checks
// the agent_readonly credential at once.
func dbAgentStack(t *testing.T, grants ...string) (*stack, uuid.UUID, pgTarget, *clientauth.Profile) {
	t.Helper()
	s, _, p := agentStack(t)
	tg := newPGTarget(t, grants...)
	db := s.addDB(t, tg)
	_, _, err := s.startGateway(t, s.token)
	require.NoError(t, err)
	return s, db, tg, p
}

// readonlyStatus waits up to 120s for the agent_readonly status of db to be
// want and returns the resource. A check that ends after the heartbeat took
// the health result of its pass is reported with the next pass of an
// instance (every 60s ±20%).
func (s *stack) readonlyStatus(t *testing.T, db uuid.UUID, want string) *werbotv1.Resource {
	t.Helper()
	var r *werbotv1.Resource
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		res, err := s.api.res.GetResource(context.Background(), connect.NewRequest(&werbotv1.GetResourceRequest{Id: db.String()}))
		require.NoError(c, err)
		r = res.Msg.GetResource()
		assert.Equal(c, want, r.GetAgentReadonlyStatus())
	}, 120*time.Second, 200*time.Millisecond)
	return r
}

const denySQLMessage = "Destructive queries are blocked"

// denyDestructiveSQL creates a deny rule for destructive queries of people
// on db and waits until the gateway refuses alice's DROP TABLE of a table
// that does not exist (the database would answer 42P01). A
// snapshot holds every rule and resource, so what changed before it has
// reached the gateway too.
func (s *stack) denyDestructiveSQL(t *testing.T, db uuid.UUID, tg pgTarget) {
	t.Helper()
	s.commandRule(t, db, "deny", denySQLMessage, "destructive")
	require.Eventually(t, func() bool {
		conn, err := s.pgDial(t, tg)
		if err != nil {
			return false
		}
		defer conn.Close(context.Background()) //nolint:errcheck // probe
		_, err = conn.Exec(context.Background(), "DROP TABLE no_such_table")
		pe, ok := errors.AsType[*pgconn.PgError](err)
		return ok && pe.Code == "42501" && pe.Message == "werbot: query denied: "+denySQLMessage
	}, settle, 200*time.Millisecond)
}

// pgDial connects alice to db through the gateway, as pgConnect, returning
// the error instead of failing the test.
func (s *stack) pgDial(t *testing.T, tg pgTarget) (*pgx.Conn, error) {
	t.Helper()
	target := "db:" + strconv.Itoa(int(tg.cfg.GetPort()))
	c, _, err := s.dial(s.gwSSH, "alice+db", s.key)
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() { _ = c.Close() })
	cc, err := pgx.ParseConfig("postgres://alice@" + target + "/" + tg.cfg.GetDatabase() + "?sslmode=disable")
	require.NoError(t, err)
	cc.LookupFunc = func(_ context.Context, host string) ([]string, error) { return []string{host}, nil }
	cc.DialFunc = func(context.Context, string, string) (net.Conn, error) { return c.Dial("tcp", target) }
	return pgx.ConnectConfig(context.Background(), cc)
}

// query calls db_query with sql on db; the call must not be a tool error.
func (m *mcpClient) query(t *testing.T, sql string) dbquery.Result {
	t.Helper()
	res := m.call(t, "db_query", map[string]any{"resource": "db", "query": sql})
	require.False(t, res.IsError, "unexpected tool error: %s", toolText(res))
	raw, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)
	var out dbquery.Result
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

// pendingQuery waits up to 5s for the pending command approval of sql and returns its id.
func (s *stack) pendingQuery(t *testing.T, sql string) string {
	t.Helper()
	var id string
	require.Eventually(t, func() bool {
		return s.pool.QueryRow(context.Background(),
			"SELECT id::text FROM approval WHERE status = 'pending' AND command = $1", sql).Scan(&id) == nil
	}, 5*time.Second, 50*time.Millisecond)
	return id
}

// queryLine finds the querylog line of sql among lines.
func queryLine(lines []querylogLine, sql string) (querylogLine, bool) {
	for _, l := range lines {
		if l.SQL == sql {
			return l, true
		}
	}
	return querylogLine{}, false
}
