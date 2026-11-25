//go:build unix

package app

import (
	"context"
	"crypto/rand"
	"io"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/gateway/telnettest"
)

// terminateWithin bounds the end of a terminated session after the RPC.
const terminateWithin = 2 * time.Second

func TestE2EWebTerminal(t *testing.T) {
	s := startStack(t)
	srv := s.sshReady(t, "srv")

	w := dialWS(t, s, "/api/terminal?resource=srv&cols=100&rows=30")
	w.send("echo hi\r")
	w.expect("echo hi")
	sid := s.liveSession(t, "web", srv)
	require.NoError(t, w.conn.Close(websocket.StatusNormalClosure, ""))

	_, reason := sessionEnd(t, s.pool, uuid.MustParse(s.alice), srv)
	assert.Equal(t, "closed", reason)
	var front, kind, ip string
	require.NoError(t, s.pool.QueryRow(context.Background(), "SELECT front, kind, host(client_ip) FROM session WHERE id = $1",
		sid).Scan(&front, &kind, &ip))
	assert.Equal(t, []string{"web", "terminal", "127.0.0.1"}, []string{front, kind, ip})
	assert.Contains(t, s.recording(t, sid.String()), "echo hi", "the web session is recorded and verified")
}

// An existing resource whose pool has no online instance closes with 1011
// "Gateway is unavailable" (owner decision O1); an unknown one looks denied.
// Neither issues a ticket or starts a session.
func TestE2EWebTerminalUnavailable(t *testing.T) {
	s := startStack(t)
	ctx := context.Background()
	gw, err := s.api.gws.CreateGateway(ctx, connect.NewRequest(&werbotv1.CreateGatewayRequest{Name: "offline"}))
	require.NoError(t, err)
	far := s.createResource(t, uuid.MustParse(gw.Msg.GetGateway().GetId()), "far", "ssh", s.sshConfigOf(t), "", true)

	code, reason := dialWS(t, s, "/api/terminal?resource=far").closeStatus()
	assert.Equal(t, websocket.StatusInternalError, code)
	assert.Equal(t, "Gateway is unavailable", reason)
	code, reason = dialWS(t, s, "/api/terminal?resource=nope").closeStatus()
	assert.Equal(t, websocket.StatusCode(4003), code)
	assert.Equal(t, "denied", reason)

	assert.Zero(t, s.count(t, "SELECT count(*) FROM stream_ticket"))
	assert.Zero(t, s.count(t, "SELECT count(*) FROM session WHERE resource_id = $1", far))
}

// A resource without a binding and an unknown resource both close with 4003
// "denied" and start no session; the gateway tells the first "Access denied".
func TestE2EWebTerminalDenied(t *testing.T) {
	s := startStack(t)
	db := s.createResource(t, s.gatewayID, "db", "ssh", s.sshConfigOf(t), s.target.Password, false)
	s.sshReady(t, "srv")

	w := dialWS(t, s, "/api/terminal?resource=db")
	w.expect("Access denied\r\n")
	code, reason := w.closeStatus()
	assert.Equal(t, websocket.StatusCode(4003), code)
	assert.Equal(t, "denied", reason)
	unknown := dialWS(t, s, "/api/terminal?resource=nope")
	code, reason = unknown.closeStatus()
	assert.Equal(t, websocket.StatusCode(4003), code)
	assert.Equal(t, "denied", reason)
	assert.Empty(t, unknown.out.String())

	assert.Zero(t, s.count(t, "SELECT count(*) FROM session WHERE resource_id = $1 OR front = 'web'", db))
}

// A web terminal on a telnet resource logs in with the stored password; the
// password reaches neither the browser nor the recording.
func TestE2EWebTerminalTelnet(t *testing.T) {
	s := startStack(t)
	tel := telnettest.Start(t, "werbot-telnet-secret-"+rand.Text())
	router := s.createResource(t, s.gatewayID, "router", "telnet", telnetConfigOf(t, tel.Addr, telnettest.Login), tel.Password, true)
	s.sshReady(t, "srv")

	w := dialWS(t, s, "/api/terminal?resource=router")
	w.expect("Welcome\r\n$ ")
	w.send("hello-telnet\r")
	w.expect("hello-telnet")
	sid := s.liveSession(t, "web", router)
	require.NoError(t, w.conn.Close(websocket.StatusNormalClosure, ""))

	_, reason := sessionEnd(t, s.pool, uuid.MustParse(s.alice), router)
	assert.Equal(t, "closed", reason)
	assert.Contains(t, tel.Shell(), "hello-telnet", "the input reached the target")
	cast := s.recording(t, sid.String())
	assert.Contains(t, cast, "hello-telnet")
	assert.NotContains(t, cast, tel.Password)
	assert.NotContains(t, w.out.String(), tel.Password)
}

// A live view shows the session's window and output, never passes the
// observer's input to the target and tells the observed user nothing (owner
// decision O3); it is audited, and an ended session has no live view.
func TestE2ELiveObserve(t *testing.T) {
	s := startStack(t)
	box := s.sshReady(t, "box")
	c := s.console(t, s.gwSSH, "alice+box")
	c.send("echo ready")
	c.expect("echo ready\r\n")
	mark := len(c.out.String())
	sid := s.liveSession(t, "ssh", box)

	w := dialWS(t, s, "/api/sessions/"+sid.String()+"/live")
	w.expectText(`{"type":"resize","cols":120,"rows":40}`)
	c.send("echo live")
	w.expect("echo live")
	w.send("echo injected\r")
	c.send("echo after")
	c.expect("echo after\r\n")
	w.expect("echo after")
	assert.Equal(t, "echo live\r\necho after\r\n", c.out.String()[mark:], "the observed terminal shows only its own output")
	assert.Equal(t, 1, s.count(t, "SELECT count(*) FROM audit_event WHERE action = 'session.observe' AND target_id = $1", sid.String()))

	require.NoError(t, c.stdin.(io.Closer).Close())
	code, _ := w.closeStatus()
	assert.Equal(t, websocket.StatusNormalClosure, code)
	assert.NotContains(t, w.out.String(), "injected")
	sessionEnd(t, s.pool, uuid.MustParse(s.alice), box)
	code, reason := dialWS(t, s, "/api/sessions/"+sid.String()+"/live").closeStatus()
	assert.Equal(t, websocket.StatusCode(4004), code)
	assert.Equal(t, "not_found", reason)
}

// A terminated SSH session ends within 2s as a gateway cut: the user sees
// only "Session closed: terminated", the reason stays in the audit (owner
// decision O2); a second terminate finds it ended.
func TestE2ETerminate(t *testing.T) {
	s := startStack(t)
	box := s.sshReady(t, "box")
	c := s.console(t, s.gwSSH, "alice+box")
	c.send("echo ready")
	c.expect("echo ready\r\n")
	sid := s.liveSession(t, "ssh", box)

	const why = "maintenance window e2e"
	start := time.Now()
	require.NoError(t, s.terminate(t, sid, why))
	assert.Equal(t, -1, c.close(), "a gateway cut has no exit status")
	assert.Less(t, time.Since(start), terminateWithin)
	assert.Contains(t, c.out.String(), "Session closed: terminated")
	assert.NotContains(t, c.out.String(), why)

	_, reason := sessionEnd(t, s.pool, uuid.MustParse(s.alice), box)
	assert.Equal(t, "terminated", reason)
	assert.Equal(t, 1, s.count(t, `SELECT count(*) FROM audit_event WHERE action = 'session.terminate' AND target_id = $1
		AND data->>'reason' = $2`, sid.String(), why))
	err := s.terminate(t, sid, why)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	assert.ErrorContains(t, err, "session already ended")
}

func TestE2ETerminateWebSession(t *testing.T) {
	s := startStack(t)
	srv := s.sshReady(t, "srv")
	w := dialWS(t, s, "/api/terminal?resource=srv")
	w.send("echo ready\r")
	w.expect("echo ready")
	sid := s.liveSession(t, "web", srv)

	start := time.Now()
	require.NoError(t, s.terminate(t, sid, "e2e"))
	code, _ := w.closeStatus()
	assert.Less(t, time.Since(start), terminateWithin)
	assert.Equal(t, websocket.StatusNormalClosure, code)
	assert.Contains(t, w.out.String(), "Session closed: terminated")
	_, reason := sessionEnd(t, s.pool, uuid.MustParse(s.alice), srv)
	assert.Equal(t, "terminated", reason)
}

func TestE2EObserverSeesWebSession(t *testing.T) {
	s := startStack(t)
	srv := s.sshReady(t, "srv")
	w := dialWS(t, s, "/api/terminal?resource=srv&cols=100&rows=30")
	w.send("echo ready\r")
	w.expect("echo ready")
	sid := s.liveSession(t, "web", srv)

	o := dialWS(t, s, "/api/sessions/"+sid.String()+"/live")
	o.expectText(`{"type":"resize","cols":100,"rows":30}`)
	w.send("echo seen\r")
	w.expect("echo seen")
	o.expect("echo seen")
}

// A console session (stage 11b) is observed like a shell, terminated like
// one, and its recording keeps the console's markers.
func TestE2ELiveObserveConsole(t *testing.T) {
	s, srv, _ := shellStack(t)
	s.terminalRule(t, srv, "console")
	require.Eventually(t, func() bool {
		out, code := s.pty(t, "echo hi")
		return out == "Interactive shell is not allowed on srv\r\n" && code == 1
	}, settle, 100*time.Millisecond)
	c := s.console(t, s.gwSSH, "alice+srv")
	c.expect("alice@srv:~$ ")
	sid := s.liveSession(t, "ssh", srv)

	w := dialWS(t, s, "/api/sessions/"+sid.String()+"/live")
	w.expectText(`{"type":"resize","cols":120,"rows":40}`)
	c.send("echo hi")
	c.expect("\r\nhi\r\n")
	w.expect("echo hi")
	w.expect("\r\nhi\r\n")
	w.expect("alice@srv:~$ ")

	start := time.Now()
	require.NoError(t, s.terminate(t, sid, "e2e"))
	assert.Equal(t, -1, c.close())
	assert.Less(t, time.Since(start), terminateWithin)
	assert.Contains(t, c.out.String(), "Session closed: terminated")
	_, reason := sessionEnd(t, s.pool, uuid.MustParse(s.alice), srv)
	assert.Equal(t, "terminated", reason)
	assert.Contains(t, s.recording(t, sid.String()), `"m","echo hi [read]"`)
}
