package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/metrics"
)

// A postgres session through the pool gateway: the client logs in without a
// password, the gateway with the resource's; every operation that reached the
// database is a line of the session's verified querylog, and the password is
// in no log, querylog or audit event.
func TestPostgresThroughGateway(t *testing.T) {
	s, hc := startStackEnv(t, nil)
	logs := s.loggedGateway(t)
	_, url := dbtest.New(t) // the target: a database of its own on the test server
	pc, err := pgconn.ParseConfig(url)
	require.NoError(t, err)
	secret := pc.Password // the resource password; it must not leak
	cfg := &postgresv1.Config{Host: pc.Host, Port: uint32(pc.Port), Database: pc.Database, Username: pc.User, Sslmode: "disable"}
	res := s.createResource(t, s.gatewayID, "pg", "postgres", cfg, secret, true)
	closed := s.createResource(t, s.gatewayID, "pg-closed", "postgres", cfg, secret, false)
	s.resourceReady(t, "pg")
	ctx := context.Background()

	target := ":" + strconv.Itoa(int(pc.Port)) // the client names the resource and its port
	conn := s.pgConnect(t, "pg"+target, pc.Database)
	pgOperations(t, conn)
	require.NoError(t, conn.Close(ctx))

	sid := endedSession(t, s, res)
	lines := s.querylog(t, hc, sid, secret)
	kinds := map[string]int{}
	for _, l := range lines {
		kinds[l.Kind]++
	}
	assert.Equal(t, map[string]int{"query": 4, "execute": 3, "cancel": 1}, kinds)
	var queries int64
	var dbName, format string
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT queries FROM session WHERE id = $1", sid).Scan(&queries))
	assert.EqualValues(t, len(lines), queries, "session.queries counts the querylog lines")
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT data->>'db_name' FROM audit_event
		WHERE action = 'session.start' AND data->>'session_id' = $1`, sid.String()).Scan(&dbName))
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT format FROM recording WHERE session_id = $1", sid).Scan(&format))
	assert.Equal(t, [2]string{pc.Database, "querylog"}, [2]string{dbName, format})
	assert.Equal(t, 1, sessionCount(t, s, res), "the cancel request opened no session")

	var leaked int
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT count(*) FROM audit_event WHERE strpos(data::text, $1) > 0", secret).Scan(&leaked))
	gwLog := logs.String()
	if gwLog == "" {
		t.Fatal("the gateway log is not captured")
	}
	if leaked != 0 || strings.Contains(gwLog, secret) {
		t.Fatal("the resource password reached the audit log or the gateway log")
	}

	t.Run("not bound", func(t *testing.T) {
		c, _, err := s.dial(s.gwSSH, "alice+pg-closed", s.key)
		require.NoError(t, err)
		defer c.Close() //nolint:errcheck // test client
		_, err = c.Dial("tcp", "pg-closed"+target)
		// refused at the channel open: PreSession runs only on an accepted channel
		oe, ok := errors.AsType[*gossh.OpenChannelError](err)
		require.True(t, ok, "not a channel refusal: %v", err)
		assert.Equal(t, "Access denied", oe.Message)
		assert.Zero(t, sessionCount(t, s, closed))
	})
}

// pgOperations runs on conn: a simple query, a pgx batch, COPY FROM and a
// query canceled through a second channel — 4 query, 3 execute and 1 cancel
// lines.
func pgOperations(t *testing.T, conn *pgx.Conn) {
	t.Helper()
	ctx := context.Background()
	pg := conn.PgConn()
	_, err := pg.Exec(ctx, "create temp table t (n int)").ReadAll()
	require.NoError(t, err)
	b := &pgx.Batch{}
	for i := range 3 {
		b.Queue("select $1::int", i)
	}
	require.NoError(t, conn.SendBatch(ctx, b).Close())
	tag, err := pg.CopyFrom(ctx, strings.NewReader("1\n2\n3\n"), "copy t from stdin")
	require.NoError(t, err)
	assert.EqualValues(t, 3, tag.RowsAffected())
	res, err := pg.Exec(ctx, "select count(*) from t").ReadAll()
	require.NoError(t, err)
	assert.Equal(t, "3", string(res[0].Rows[0][0]))

	errc := make(chan error, 1)
	go func() { _, err := pg.Exec(ctx, "select pg_sleep(30)").ReadAll(); errc <- err }()
	time.Sleep(300 * time.Millisecond) // the query is running
	require.NoError(t, pg.CancelRequest(ctx))
	select {
	case err := <-errc:
		pe, ok := errors.AsType[*pgconn.PgError](err)
		require.True(t, ok, "not a server error: %v", err)
		assert.Equal(t, "57014", pe.Code)
	case <-time.After(10 * time.Second):
		t.Fatal("the query was not canceled")
	}
}

// pgConnect connects to db at target (<resource>:<port>) through the gateway
// as alice: every connection, the cancel request's too, is a direct-tcpip
// channel of one SSH connection.
func (s *stack) pgConnect(t *testing.T, target, db string) *pgx.Conn {
	t.Helper()
	resource, _, err := net.SplitHostPort(target)
	require.NoError(t, err)
	c, _, err := s.dial(s.gwSSH, "alice+"+resource, s.key)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	cc, err := pgx.ParseConfig("postgres://alice@" + target + "/" + db + "?sslmode=disable")
	require.NoError(t, err)
	cc.LookupFunc = func(_ context.Context, host string) ([]string, error) { return []string{host}, nil }
	cc.DialFunc = func(context.Context, string, string) (net.Conn, error) { return c.Dial("tcp", target) }
	conn, err := pgx.ConnectConfig(context.Background(), cc)
	require.NoError(t, err)
	return conn
}

// loggedGateway runs a second instance of the pool "gw" with a debug log and
// makes it the stack's SSH address; it returns the log.
func (s *stack) loggedGateway(t *testing.T) *syncBuffer {
	t.Helper()
	logs := &syncBuffer{}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NoError(t, runUntilUp(t, func(ctx context.Context, reg *metrics.Registry) error {
		cfg := gateway.Config{ServerURL: s.url, JoinToken: s.token, DataDir: t.TempDir(), Name: "gw-logged",
			IdleTimeout: time.Minute, DrainTimeout: time.Second, MaxSessions: 100, Metrics: reg}
		return gateway.Serve(ctx, cfg, ln, slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}))
	s.gwSSH = ln.Addr().String()
	return logs
}

// sessionCount is the number of sessions of resource.
func sessionCount(t *testing.T, s *stack, resource uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, s.pool.QueryRow(context.Background(), "SELECT count(*) FROM session WHERE resource_id = $1", resource).Scan(&n))
	return n
}

// endedSession waits up to 20s for the one session of resource to end.
func endedSession(t *testing.T, s *stack, resource uuid.UUID) uuid.UUID {
	t.Helper()
	var sid uuid.UUID
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.NoError(c, s.pool.QueryRow(context.Background(),
			"SELECT id FROM session WHERE resource_id = $1 AND ended_at IS NOT NULL", resource).Scan(&sid))
	}, 20*time.Second, 100*time.Millisecond)
	return sid
}

// querylogLine is what the test reads of a querylog line.
type querylogLine struct {
	Kind string `json:"kind"`
}

// querylog waits up to 20s for the verified querylog of session sid; the
// resource password is not in it.
func (s *stack) querylog(t *testing.T, hc *http.Client, sid uuid.UUID, secret string) []querylogLine {
	t.Helper()
	var body []byte
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, s.url+"/api/recordings/"+sid.String()+".querylog", nil)
		require.NoError(c, err)
		res, err := hc.Do(req)
		require.NoError(c, err)
		defer res.Body.Close() //nolint:errcheck // test client
		body, err = io.ReadAll(res.Body)
		require.NoError(c, err)
		require.Equal(c, http.StatusOK, res.StatusCode)
		assert.Equal(c, "application/x-ndjson", res.Header.Get("Content-Type"))
		assert.Equal(c, "true", res.Header.Get("X-Werbot-Recording-Verified"))
	}, 20*time.Second, 200*time.Millisecond)
	if bytes.Contains(body, []byte(secret)) {
		t.Fatal("the resource password reached the querylog")
	}
	var lines []querylogLine
	sc := bufio.NewScanner(bytes.NewReader(body))
	for sc.Scan() {
		var l querylogLine
		require.NoError(t, json.Unmarshal(sc.Bytes(), &l), "a querylog line is JSON")
		lines = append(lines, l)
	}
	return lines
}
