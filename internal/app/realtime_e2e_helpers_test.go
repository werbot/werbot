//go:build unix

package app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	sshv1 "github.com/werbot/werbot/gen/werbot/connector/ssh/v1"
	telnetv1 "github.com/werbot/werbot/gen/werbot/connector/telnet/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
)

// wsWait bounds every blocking step of a browser WebSocket, so a regression
// fails fast instead of hanging until -timeout.
const wsWait = 10 * time.Second

// wsConn is a browser WebSocket of alice; a goroutine collects what the
// server sends until the connection closes.
type wsConn struct {
	t      *testing.T
	conn   *websocket.Conn
	out    syncBuffer    // binary frames: terminal output
	text   syncBuffer    // text frames, one per line
	read   int           // output bytes already matched by expect
	closed chan struct{} // closed when the server closed the connection
	status websocket.StatusCode
	reason string
}

// dialWS opens the WebSocket path on the stack as alice, with the stack's
// Origin, and fails the test unless the upgrade succeeds.
func dialWS(t *testing.T, s *stack, path string) *wsConn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), wsWait)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(s.url, "http")+path, &websocket.DialOptions{ //nolint:bodyclose // websocket.Dial owns the body
		HTTPClient: s.aliceHTTP(t), HTTPHeader: http.Header{"Origin": {s.url}}})
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.CloseNow() })
	c := &wsConn{t: t, conn: conn, closed: make(chan struct{})}
	go c.collect()
	return c
}

func (c *wsConn) collect() {
	defer close(c.closed)
	for {
		typ, b, err := c.conn.Read(context.Background())
		if err != nil {
			var ce websocket.CloseError
			if errors.As(err, &ce) {
				c.status, c.reason = ce.Code, ce.Reason
			}
			return
		}
		if typ == websocket.MessageText {
			_, _ = c.text.Write(append(b, '\n'))
		} else {
			_, _ = c.out.Write(b)
		}
	}
}

// expect waits for want in the output after the last match.
func (c *wsConn) expect(want string) {
	c.t.Helper()
	var got string
	if !assert.Eventually(c.t, func() bool {
		got = c.out.String()[c.read:]
		return strings.Contains(got, want)
	}, wsWait, 10*time.Millisecond) {
		c.t.Fatalf("websocket output lacks %q: %q", want, got)
	}
	c.read += strings.Index(got, want) + len(want)
}

// expectText waits for the text frame line.
func (c *wsConn) expectText(line string) {
	c.t.Helper()
	require.Eventually(c.t, func() bool { return strings.Contains(c.text.String(), line+"\n") }, wsWait, 10*time.Millisecond,
		"text frames: %q", c.text.String())
}

// send writes p as one binary frame, as the browser's keystrokes.
func (c *wsConn) send(p string) {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), wsWait)
	defer cancel()
	require.NoError(c.t, c.conn.Write(ctx, websocket.MessageBinary, []byte(p)))
}

// closeStatus waits for the server to close the connection and returns its code and reason.
func (c *wsConn) closeStatus() (websocket.StatusCode, string) {
	c.t.Helper()
	select {
	case <-c.closed:
	case <-time.After(wsWait):
		c.t.Fatalf("websocket still open after %v", wsWait)
	}
	return c.status, c.reason
}

// sessionsAPI is the SessionService client of alice.
func (s *stack) sessionsAPI(t *testing.T) werbotv1connect.SessionServiceClient {
	t.Helper()
	return werbotv1connect.NewSessionServiceClient(s.aliceHTTP(t), s.url)
}

// terminate asks the server to end session sid with reason.
func (s *stack) terminate(t *testing.T, sid uuid.UUID, reason string) error {
	t.Helper()
	_, err := s.sessionsAPI(t).TerminateSession(context.Background(), connect.NewRequest(&werbotv1.TerminateSessionRequest{
		Id: sid.String(), Reason: reason}))
	return err
}

// liveSession waits for the open terminal session of front on resource
// that a gateway instance runs and returns its id.
func (s *stack) liveSession(t *testing.T, front string, resource uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.NoError(c, s.pool.QueryRow(context.Background(), `SELECT id FROM session WHERE front = $1 AND resource_id = $2
			AND kind = 'terminal' AND ended_at IS NULL AND gateway_instance_id IS NOT NULL`, front, resource).Scan(&id))
	}, wsWait, 50*time.Millisecond)
	return id
}

// count runs a count(*) query.
func (s *stack) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, s.pool.QueryRow(context.Background(), query, args...).Scan(&n))
	return n
}

// createResource creates resource name of typ with cfg in pool gatewayID,
// with password when not empty, bound to alice when bind is set.
func (s *stack) createResource(t *testing.T, gatewayID uuid.UUID, name, typ string, cfg proto.Message, password string, bind bool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	a, err := anypb.New(cfg)
	require.NoError(t, err)
	res, err := s.api.res.CreateResource(ctx, connect.NewRequest(&werbotv1.CreateResourceRequest{
		Name: name, Type: typ, GatewayId: gatewayID.String(), Config: a}))
	require.NoError(t, err)
	id := res.Msg.GetResource().GetId()
	if password != "" {
		_, err = s.api.res.SetCredential(ctx, connect.NewRequest(&werbotv1.SetCredentialRequest{
			ResourceId: id, Kind: "password", Value: []byte(password)}))
		require.NoError(t, err)
	}
	if bind {
		_, err = s.api.acc.CreateBinding(ctx, connect.NewRequest(&werbotv1.CreateBindingRequest{
			SubjectType: "user", SubjectId: s.alice, ResourceId: id}))
		require.NoError(t, err)
	}
	return uuid.MustParse(id)
}

// sshConfigOf is the config of an ssh resource at the stack's target.
func (s *stack) sshConfigOf(t *testing.T) *sshv1.Config {
	t.Helper()
	host, port, err := net.SplitHostPort(s.target.Addr)
	require.NoError(t, err)
	p, err := strconv.ParseUint(port, 10, 16)
	require.NoError(t, err)
	return &sshv1.Config{Host: host, Port: uint32(p), Login: "root"}
}

// telnetConfigOf is the config of a telnet resource at addr logging in as login.
func telnetConfigOf(t *testing.T, addr, login string) *telnetv1.Config {
	t.Helper()
	host, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	p, err := strconv.ParseUint(port, 10, 16)
	require.NoError(t, err)
	return &telnetv1.Config{Host: host, Port: uint32(p), Login: login}
}

// sshReady adds the ssh resource name bound to alice and waits until the
// gateway runs a command on it: every resource created before it has
// reached the gateway too.
func (s *stack) sshReady(t *testing.T, name string) uuid.UUID {
	t.Helper()
	id := s.addResource(t, name, "ssh")
	require.Eventually(t, func() bool { return s.allowed(t, s.key, "alice+"+name) }, settle, 100*time.Millisecond)
	return id
}
