//go:build cluster

package cluster

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

const (
	terminalPath = "/api/terminal?resource=target&cols=80&rows=24"
	readMax      = 10 * time.Second
)

// typeMark types a command that prints "mark:<mark>"; only the command output
// carries "mark:<mark>\r", the echoed input reads "mark:%s".
func typeMark(t testing.TB, c *websocket.Conn, mark string) {
	t.Helper()
	cmd := fmt.Sprintf("printf 'mark:%%s\\n' %s\r", mark)
	require.NoError(t, c.Write(context.Background(), websocket.MessageBinary, []byte(cmd)))
}

// relayStreams is werbot_relay_streams of a server replica.
func relayStreams(t testing.TB, service string) int64 {
	t.Helper()
	return Metric(t, st.Addr(t, service, 9100), "werbot_relay_streams")
}

// auditCount counts audit events of action on session id.
func auditCount(t testing.TB, action string, id uuid.UUID, extra string) (n int) {
	t.Helper()
	q := `SELECT count(*) FROM audit_event WHERE action = $1 AND target_id = $2 ` + extra
	require.NoError(t, st.DB.QueryRow(context.Background(), q, action, id.String()).Scan(&n))
	return n
}

// TestScenario15WebTerminalAcrossReplicas: a browser on server2 reaches a
// session on gw1 (tunnel on server1) through a bridge between the replicas; the
// same works through the UI (werbot web and nginx). Both sessions are web
// sessions, end as closed and keep a verified recording.
func TestScenario15WebTerminalAcrossReplicas(t *testing.T) {
	st.Healthy(t)

	c := st.DialWSVia(t, st.B, terminalPath, "gw1")
	typeMark(t, c, "web-mark")
	ReadUntil(t, c, "mark:web-mark\r", readMax)
	require.Eventually(t, func() bool { return relayStreams(t, "server2") == 1 && relayStreams(t, "server1") == 1 },
		5*time.Second, 100*time.Millisecond, "one stream on the browser replica and one bridge on the tunnel replica")
	id := st.OpenSessionOn(t, "gw1")
	require.NoError(t, c.Close(websocket.StatusNormalClosure, ""))
	st.WaitEnded(t, id, "closed", UploadDrain)
	st.WaitVerified(t, id, UploadDrain)

	ui := st.DialWSVia(t, st.UI, terminalPath, "gw1")
	typeMark(t, ui, "ui-mark")
	ReadUntil(t, ui, "mark:ui-mark\r", readMax)
	uiID := st.OpenSessionOn(t, "gw1")
	require.NoError(t, ui.Close(websocket.StatusNormalClosure, ""))
	st.WaitEnded(t, uiID, "closed", UploadDrain)
	st.WaitVerified(t, uiID, UploadDrain)

	for _, sid := range []uuid.UUID{id, uiID} {
		var front string
		require.NoError(t, st.DB.QueryRow(context.Background(), `SELECT front FROM session WHERE id = $1`, sid).Scan(&front))
		assert.Equal(t, "web", front, "front of session %s", sid)
	}
}

// TestScenario15LiveViewAcrossReplicas: an observer on server2 sees the output
// of an SSH session on gw1; its input never reaches the observed shell; the
// observation is audited.
func TestScenario15LiveViewAcrossReplicas(t *testing.T) {
	st.Healthy(t)
	sh := OpenShell(t, st.Dial(t, st.Addr(t, "nginx", 2201), "alice+target"))
	sh.Mark(t, "ready")
	id := st.OpenSessionOn(t, "gw1")

	c := st.DialWS(t, st.B, "/api/sessions/"+id.String()+"/live")
	ctx, cancel := context.WithTimeout(context.Background(), readMax)
	defer cancel()
	typ, first, err := c.Read(ctx) // the window message: the observer is attached
	require.NoError(t, err)
	require.Equal(t, websocket.MessageText, typ)
	require.Contains(t, string(first), `"resize"`)

	sh.Mark(t, "live-mark")
	ReadUntil(t, c, "mark:live-mark\r", readMax)
	before := sh.out.String()
	require.NoError(t, c.Write(ctx, websocket.MessageBinary, []byte("printf 'mark:%s\\n' injected\r")))
	sh.Mark(t, "after-input") // the observer's bytes were sent before this mark: they would show first
	assert.NotContains(t, sh.out.String()[len(before):], "mark:injected", "a live view never forwards browser input")
	assert.Equal(t, 1, auditCount(t, "session.observe", id, ""))
}

// TestScenario16TerminateFromOtherReplica: terminate called on server2 ends a
// session whose tunnel is on server1 within TerminateMax; the user sees only
// "Session closed: terminated" and the audit keeps the reason.
func TestScenario16TerminateFromOtherReplica(t *testing.T) {
	st.Healthy(t)
	sh := OpenShell(t, st.Dial(t, st.Addr(t, "nginx", 2201), "alice+target"))
	sh.Mark(t, "ready")
	id := st.OpenSessionOn(t, "gw1")

	t0 := time.Now()
	_, err := st.B.Sessions.TerminateSession(context.Background(), connect.NewRequest(&werbotv1.TerminateSessionRequest{Id: id.String(), Reason: "cluster"}))
	require.NoError(t, err)
	select {
	case <-sh.done:
	case <-time.After(TerminateMax):
		t.Fatalf("shell still open %s after terminate; output:\n%s", TerminateMax, sh.out.String())
	}
	t.Logf("terminate %s", time.Since(t0))
	assert.Contains(t, sh.out.String(), "Session closed: terminated")
	assert.NotContains(t, sh.out.String(), "cluster", "the reason stays in the audit")

	st.WaitEnded(t, id, "terminated", 5*time.Second)
	assert.Equal(t, 1, auditCount(t, "session.terminate", id, `AND data->>'reason' = 'cluster'`))
}

// TestScenario15KillBridgeReplica: when the replica that holds the bridge dies,
// the browser closes with 1011 within RelayCloseMax, the web session ends as
// error and an SSH session on the other gateway keeps working.
func TestScenario15KillBridgeReplica(t *testing.T) {
	st.Healthy(t)
	keep := OpenShell(t, st.Dial(t, st.Addr(t, "nginx", 2202), "alice+target")) // gw2 -> server2
	keep.Mark(t, "ssh-before")
	c := st.DialWSVia(t, st.B, terminalPath, "gw1") // browser on server2, bridge on server1
	typeMark(t, c, "bridge-up")
	ReadUntil(t, c, "mark:bridge-up\r", readMax)
	id := st.OpenSessionOn(t, "gw1")

	t.Cleanup(func() {
		_, err := st.Compose(context.Background(), "start", "server1")
		assert.NoError(t, err)
		st.Healthy(t)
	})
	_, err := st.Compose(context.Background(), "kill", "server1")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), RelayCloseMax)
	defer cancel()
	for err == nil {
		_, _, err = c.Read(ctx)
	}
	assert.Equal(t, websocket.StatusInternalError, websocket.CloseStatus(err), "close of the browser: %v", err)
	assert.False(t, strings.Contains(err.Error(), "deadline"), "closed within %s: %v", RelayCloseMax, err)

	keep.Mark(t, "ssh-alive")
	st.WaitEnded(t, id, "error", UploadDrain)
}
