package websocket

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/utils/test"
)

func TestHandler_websocket(t *testing.T) {
	app, teardownTestCase := test.API(t)
	defer teardownTestCase(t)

	New(app.Handler).Routes()

	// A normal HTTP request must not reach the WebSocket handler.
	response, err := app.App.Test(httptest.NewRequest(http.MethodGet, "/ws/00000000-0000-0000-0000-000000000001", nil), -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusUpgradeRequired, response.StatusCode)
	_ = response.Body.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	go func() {
		_ = app.App.Listener(listener)
	}()
	defer app.App.Shutdown()

	dial := func(sessionID string) *websocket.Conn {
		t.Helper()

		url := "ws://" + listener.Addr().String() + "/ws/" + sessionID
		var conn *websocket.Conn
		for attempt := 0; attempt < 20; attempt++ {
			conn, _, err = websocket.DefaultDialer.Dial(url, nil)
			if err == nil {
				return conn
			}
			time.Sleep(25 * time.Millisecond)
		}
		require.NoError(t, err)
		return nil
	}

	// Unknown sessions are rejected before a subscription is created.
	invalid := dial("00000000-0000-0000-0000-000000000002")
	defer invalid.Close()

	_, payload, err := invalid.ReadMessage()
	require.NoError(t, err)
	var invalidResponse Response
	require.NoError(t, json.Unmarshal(payload, &invalidResponse))
	require.Equal(t, http.StatusUnauthorized, invalidResponse.Code)
	require.Equal(t, "error", invalidResponse.Action)

	// Register a valid session in the same Redis store the route reads.
	const (
		sessionID = "00000000-0000-0000-0000-000000000003"
		profileID = test.ConstAdminID
	)
	ctx := context.Background()
	require.NoError(t, app.Redis.Client.HSet(ctx, "refresh_token:"+sessionID, "profile_id", profileID).Err())

	conn := dial(sessionID)
	defer conn.Close()

	_, payload, err = conn.ReadMessage()
	require.NoError(t, err)
	var subscribed Response
	require.NoError(t, json.Unmarshal(payload, &subscribed))
	require.Equal(t, http.StatusOK, subscribed.Code)
	require.Equal(t, "subscribe", subscribed.Action)

	// The info action writes the UI and API versions.
	require.NoError(t, conn.WriteJSON(Request{Action: "info"}))
	_, payload, err = conn.ReadMessage()
	require.NoError(t, err)
	var versions map[string]string
	require.NoError(t, json.Unmarshal(payload, &versions))
	require.Contains(t, versions, "ui")
	require.Contains(t, versions, "api")

	// Unknown actions get an error response.
	require.NoError(t, conn.WriteJSON(Request{Action: "unknown"}))
	_, payload, err = conn.ReadMessage()
	require.NoError(t, err)
	var unknown Response
	require.NoError(t, json.Unmarshal(payload, &unknown))
	require.Equal(t, http.StatusBadRequest, unknown.Code)
	require.Equal(t, "error", unknown.Action)

	// Redis pub/sub messages are forwarded to the connected client.
	require.NoError(t, app.Redis.Client.Publish(ctx, "ws:"+profileID, "hello").Err())
	_, payload, err = conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "hello", string(payload))
}
