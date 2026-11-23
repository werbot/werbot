package relay

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
)

var live = "/api/sessions/" + uuid.NewString() + "/live"

func TestWebSocketOriginRejected(t *testing.T) {
	r, spy := idleRelay(t, nil)
	srv := serveWS(t, r, &member)
	origins := []string{"", "null", "https://evil.example", "http://ui.example.com", "https://ui.example.com:8443"}
	for i, origin := range origins {
		path := []string{"/api/terminal", live}[i%2]
		assert.Equal(t, http.StatusForbidden, refused(t, srv, path, origin), "origin %q", origin)
	}
	assert.Empty(t, spy.list(), "no ticket for a foreign origin")
	assert.EqualValues(t, len(origins), rejected(t, r, "origin"))

	r, _ = idleRelay(t, apperr.ErrPermissionDenied)
	r.cfg.WebURL = "https://UI.example.com:443/app"
	c, res, err := dial(t, serveWS(t, r, &member), "/api/terminal", webURL) //nolint:bodyclose // websocket.Dial owns the body
	require.NoError(t, err, "default port and case are normalized")
	assert.Equal(t, http.StatusSwitchingProtocols, res.StatusCode)
	code, reason := closed(t, c)
	assert.Equal(t, [2]any{websocket.StatusCode(4003), ReasonDenied}, [2]any{code, reason}, "the accepted stream reaches the ticket check")
	assert.Zero(t, rejected(t, r, "origin"))
}

func TestWebSocketUnauthenticated(t *testing.T) {
	r, spy := idleRelay(t, nil)
	srv := serveWS(t, r, nil)
	for _, path := range []string{"/api/terminal?resource=db", live} {
		assert.Equal(t, http.StatusUnauthorized, refused(t, srv, path, webURL), path)
	}
	assert.Empty(t, spy.list())
}

func TestWebSocketRefusesAgent(t *testing.T) {
	r, spy := idleRelay(t, nil)
	srv := serveWS(t, r, &agent)
	for _, path := range []string{"/api/terminal?resource=db", live} {
		assert.Equal(t, http.StatusForbidden, refused(t, srv, path, webURL), path)
	}
	assert.Empty(t, spy.list(), "an agent principal gets no ticket")
}

func TestTerminalBadWindow(t *testing.T) {
	r, spy := idleRelay(t, apperr.ErrPermissionDenied)
	srv := serveWS(t, r, &member)
	for _, q := range []string{"cols=abc", "cols=0", "cols=501", "rows=0", "rows=201", "rows=1.5", "cols="} {
		assert.Equal(t, http.StatusBadRequest, refused(t, srv, "/api/terminal?resource=db&"+q, webURL), q)
	}
	assert.Empty(t, spy.list(), "no ticket for a bad window")

	c := mustDial(t, srv, "/api/terminal?resource=db")
	code, _ := closed(t, c)
	assert.Equal(t, websocket.StatusCode(4003), code)
	c = mustDial(t, srv, "/api/terminal?resource=db&cols=500&rows=200")
	_, _ = closed(t, c)
	id := r.cfg.InstanceID.String()
	assert.Equal(t, []string{"terminal db 80x24 " + id, "terminal db 500x200 " + id}, spy.list())
}

func TestTerminalWebSocket(t *testing.T) {
	c, spy, g := wsRelay(t, nil)
	b := c.rs[0]
	ws := mustDial(t, serveWS(t, b, &member), "/api/terminal?resource=db&cols=120&rows=40")
	gw := peer(t, g)
	assert.Equal(t, []string{"terminal db 120x40 " + b.cfg.InstanceID.String()}, spy.list())

	send(t, ws, websocket.MessageBinary, "ls\r")
	assert.Equal(t, "ls\r", string(nextFrame(t, gw).data))
	require.NoError(t, gw.Write([]byte("out")))
	typ, msg := recv(t, ws)
	assert.Equal(t, websocket.MessageBinary, typ)
	assert.Equal(t, "out", msg)

	send(t, ws, websocket.MessageText, `{"type":"resize","cols":100,"rows":30}`)
	f := nextFrame(t, gw)
	assert.Equal(t, [2]int{100, 30}, [2]int{f.cols, f.rows})

	require.NoError(t, gw.Close(ReasonClosed))
	code, reason := closed(t, ws)
	assert.Equal(t, websocket.StatusNormalClosure, code)
	assert.Equal(t, ReasonClosed, reason)
	require.NoError(t, within(t, func() error { return <-g.results }))
	drained(t, b)
}

func TestLiveDropsBrowserInput(t *testing.T) {
	c, spy, g := wsRelay(t, nil)
	b := c.rs[0]
	ws := mustDial(t, serveWS(t, b, &member), live)
	gw := peer(t, g)
	require.Len(t, spy.list(), 1)
	assert.True(t, strings.HasPrefix(spy.list()[0], "observe "+strings.Split(live, "/")[3]))

	send(t, ws, websocket.MessageBinary, "echo injected\r")
	send(t, ws, websocket.MessageText, `{"type":"resize","cols":100,"rows":30}`)
	select {
	case f := <-gw.in:
		t.Fatalf("live view forwarded browser input: %+v", f)
	case <-time.After(300 * time.Millisecond):
	}

	require.NoError(t, gw.Resize(132, 43))
	typ, msg := recv(t, ws)
	assert.Equal(t, websocket.MessageText, typ)
	assert.JSONEq(t, `{"type":"resize","cols":132,"rows":43}`, msg)
	require.NoError(t, gw.Write([]byte("out")))
	typ, msg = recv(t, ws)
	assert.Equal(t, websocket.MessageBinary, typ)
	assert.Equal(t, "out", msg)

	require.NoError(t, ws.Close(websocket.StatusNormalClosure, ""))
	assert.Equal(t, ReasonClosed, readClose(t, gw))
	drained(t, b)
}

func TestWebSocketMalformedText(t *testing.T) {
	c, _, g := wsRelay(t, nil)
	b := c.rs[0]
	ws := mustDial(t, serveWS(t, b, &member), "/api/terminal?resource=db")
	gw := peer(t, g)
	for _, m := range []string{"{", `{"type":"x"}`, `{"type":"resize","cols":99999,"rows":24}`,
		`{"type":"resize","cols":80,"rows":0}`, `{"type":"resize","cols":1.5,"rows":24}`} {
		send(t, ws, websocket.MessageText, m)
	}
	send(t, ws, websocket.MessageBinary, "next")
	f := nextFrame(t, gw)
	assert.Zero(t, f.cols, "no resize reached the gateway")
	assert.Equal(t, "next", string(f.data), "the connection lives")

	require.NoError(t, ws.Close(websocket.StatusNormalClosure, ""))
	assert.Equal(t, ReasonClosed, readClose(t, gw))
	drained(t, b)
}

// TestWebSocketReadLimit: a message over 64 KiB closes that stream with 1009;
// another stream of the replica goes on.
func TestWebSocketReadLimit(t *testing.T) {
	c, _, g := wsRelay(t, nil)
	b := c.rs[0]
	srv := serveWS(t, b, &member)
	big, other := mustDial(t, srv, "/api/terminal?resource=db"), mustDial(t, srv, "/api/terminal?resource=db")
	gwBig, gwOther := peer(t, g), peer(t, g)

	send(t, big, websocket.MessageBinary, strings.Repeat("a", 64<<10))
	assert.Len(t, nextFrame(t, gwBig).data, 64<<10, "a 64 KiB message passes")
	send(t, big, websocket.MessageBinary, strings.Repeat("a", 64<<10+1))
	code, _ := closed(t, big)
	assert.Equal(t, websocket.StatusMessageTooBig, code)
	assert.Equal(t, ReasonClosed, readClose(t, gwBig))

	send(t, other, websocket.MessageBinary, "ok")
	assert.Equal(t, "ok", string(nextFrame(t, gwOther).data))
	require.NoError(t, other.Close(websocket.StatusNormalClosure, ""))
	assert.Equal(t, ReasonClosed, readClose(t, gwOther))
	drained(t, b)
}
