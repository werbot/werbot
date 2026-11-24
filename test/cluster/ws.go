//go:build cluster || load || redteam

package cluster

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Limits of the realtime scenarios (spec 12 §9 and §15).
const (
	TerminateMax  = 2 * time.Second  // terminate on any replica ends the shell
	RelayCloseMax = 15 * time.Second // a dead bridge replica closes the browser: ping 5 s + 5 s + margin

	wsPlacements = 8                // DialWSVia attempts: the pool instance of a ticket is random
	wsSessionMax = 15 * time.Second // the web session row appears after the stream opens (AwaitTimeout 10 s)
)

// DialWS opens the WebSocket path on api as alice with the Origin of the UI
// (WERBOT_WEB_URL). The connection closes with the test.
func (s *Stand) DialWS(t testing.TB, api API, path string) *websocket.Conn {
	t.Helper()
	c, err := dialWS(api, path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close(websocket.StatusNormalClosure, "") })
	return c
}

func dialWS(api API, path string) (*websocket.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, api.BaseURL+path, &websocket.DialOptions{ //nolint:bodyclose // Dial owns the handshake body
		HTTPClient: api.HTTP, HTTPHeader: http.Header{"Origin": {"http://" + UIHost}},
	})
	return c, err
}

// DialWSVia is DialWS for a terminal path whose web session runs on the
// gateway instance gw ("gw1"): a ticket picks a random online instance of the
// pool, so a connection that landed elsewhere closes and the dial repeats, up
// to wsPlacements times.
func (s *Stand) DialWSVia(t testing.TB, api API, path, gw string) *websocket.Conn {
	t.Helper()
	for range wsPlacements {
		before, err := s.openWebSessions(t.Context())
		require.NoError(t, err)
		c, err := dialWS(api, path)
		require.NoError(t, err)
		host := s.awaitWebSession(t, before)
		if host == gw {
			t.Cleanup(func() { _ = c.Close(websocket.StatusNormalClosure, "") })
			return c
		}
		_ = c.CloseNow()
	}
	t.Fatalf("web session did not land on %s in %d dials", gw, wsPlacements)
	return nil
}

// openWebSessions maps the open web sessions to the hostname of their gateway instance.
func (s *Stand) openWebSessions(ctx context.Context) (map[uuid.UUID]string, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT s.id, coalesce(gi.hostname, '')
		FROM session s LEFT JOIN gateway_instance gi ON gi.id = s.gateway_instance_id
		WHERE s.front = 'web' AND s.ended_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	open := map[uuid.UUID]string{}
	for rows.Next() {
		var id uuid.UUID
		var host string
		if err := rows.Scan(&id, &host); err != nil {
			return nil, err
		}
		open[id] = host
	}
	return open, rows.Err()
}

// awaitWebSession returns the instance hostname of the first open web session absent from before.
func (s *Stand) awaitWebSession(t testing.TB, before map[uuid.UUID]string) (host string) {
	t.Helper()
	err := poll(t.Context(), wsSessionMax, func(ctx context.Context) error {
		now, err := s.openWebSessions(ctx)
		if err != nil {
			return err
		}
		for id, h := range now {
			if _, old := before[id]; !old {
				host = h
				return nil
			}
		}
		return errors.New("no new web session")
	})
	require.NoError(t, err)
	return host
}

// ReadUntil reads frames of c until their joined output holds mark; it fails
// the test after timeout. The connection is unusable afterwards on a timeout.
func ReadUntil(t testing.TB, c *websocket.Conn, mark string, timeout time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var out strings.Builder
	for !strings.Contains(out.String(), mark) {
		typ, b, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("mark %q not seen within %s: %v; output:\n%s", mark, timeout, err, out.String())
		}
		if typ == websocket.MessageBinary {
			out.Write(b)
		}
	}
}
