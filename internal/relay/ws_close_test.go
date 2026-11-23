package relay

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
)

func TestWebSocketCloseCodes(t *testing.T) {
	const terminal = "/api/terminal?resource=db"
	for _, c := range []struct {
		name, path string
		err        error
		code       websocket.StatusCode
		reason     string
	}{
		{"live not found", live, apperr.ErrNotFound, 4004, ReasonNotFound},
		{"live bad id", "/api/sessions/not-a-uuid/live", nil, 4004, ReasonNotFound},
		{"terminal not found looks denied", terminal, apperr.ErrNotFound, 4003, ReasonDenied},
		{"denied", terminal, apperr.ErrPermissionDenied, 4003, ReasonDenied},
		{"live denied", live, apperr.ErrPermissionDenied, 4003, ReasonDenied},
		{"gateway unavailable", terminal, apperr.ErrUnavailable, 1011, "Gateway is unavailable"},
		{"other error", terminal, errors.New("db down"), 1011, ReasonError},
	} {
		t.Run(c.name, func(t *testing.T) {
			r, _ := idleRelay(t, c.err)
			code, reason := closed(t, mustDial(t, serveWS(t, r, &member), c.path))
			assert.Equal(t, c.code, code)
			assert.Equal(t, c.reason, reason)
			assert.Zero(t, streams(t, r))
		})
	}
	t.Run("limit", func(t *testing.T) {
		r, spy := idleRelay(t, nil)
		r.maxStreams = 0
		code, reason := closed(t, mustDial(t, serveWS(t, r, &member), terminal))
		assert.Equal(t, websocket.StatusCode(4008), code)
		assert.Equal(t, ReasonLimit, reason)
		assert.Empty(t, spy.list())
	})

	cl, _, g := wsRelay(t, func(r *Relay) { r.awaitTimeout = 200 * time.Millisecond })
	b := cl.rs[0]
	srv := serveWS(t, b, &member)
	g.via.Store(nil)
	code, reason := closed(t, mustDial(t, srv, terminal))
	assert.Equal(t, [2]any{websocket.StatusInternalError, ReasonError}, [2]any{code, reason}, "await timeout")
	g.via.Store(b)
	for reason, code := range map[string]websocket.StatusCode{ReasonDenied: 4003, ReasonNotFound: 4004, ReasonLimit: 4008,
		"mfa_required": 4001, ReasonError: 1011, ReasonClosed: 1000} {
		ws := mustDial(t, srv, terminal)
		require.NoError(t, peer(t, g).Close(reason))
		got, text := closed(t, ws)
		assert.Equal(t, code, got, reason)
		assert.Equal(t, reason, text)
	}
	drained(t, b)
}

func TestWebSocketSlotsReleased(t *testing.T) {
	cl, spy, g := wsRelay(t, func(r *Relay) { r.awaitTimeout, r.ping = time.Second, 100*time.Millisecond })
	b := cl.rs[0]
	srv := serveWS(t, b, &member)
	const terminal = "/api/terminal?resource=db"

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+terminal, nil)
	require.NoError(t, err)
	req.Header.Set("Origin", webURL)
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = res.Body.Close()
	assert.Equal(t, http.StatusUpgradeRequired, res.StatusCode, "Accept refuses a plain request")
	assert.Empty(t, spy.list())

	g.via.Store(nil)
	code, _ := closed(t, mustDial(t, srv, terminal))
	assert.Equal(t, websocket.StatusInternalError, code, "await timeout")
	drained(t, b)
	g.via.Store(b)

	ws := mustDial(t, srv, terminal)
	gw := peer(t, g)
	require.NoError(t, ws.Close(websocket.StatusGoingAway, ""))
	assert.Equal(t, ReasonClosed, gwClose(t, gw), "browser close")
	drained(t, b)

	ws = mustDial(t, srv, terminal)
	require.NoError(t, peer(t, g).Close(ReasonClosed))
	code, _ = closed(t, ws)
	assert.Equal(t, websocket.StatusNormalClosure, code, "gateway close")
	drained(t, b)

	_ = mustDial(t, srv, terminal) // never read: pings go unanswered
	assert.Equal(t, ReasonClosed, gwClose(t, peer(t, g)), "ping failure")
	drained(t, b)

	ws = mustDial(t, srv, terminal)
	gw = peer(t, g)
	readErr := make(chan error, 1)
	go func() { _, _, err := ws.Read(ctx); readErr <- err }() // answers pings while the replica stops
	require.NoError(t, cl.stops[0]())
	var ce websocket.CloseError
	require.ErrorAs(t, within(t, func() error { return <-readErr }), &ce)
	assert.Equal(t, [2]any{websocket.StatusInternalError, ReasonError}, [2]any{ce.Code, ce.Reason}, "replica shutdown")
	assert.Equal(t, ReasonError, gwClose(t, gw))
	drained(t, b)
}
