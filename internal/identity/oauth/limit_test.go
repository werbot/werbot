package oauth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/identity/oauth"
)

// freshMinute waits for the next minute when under 20 s of the current one
// remain, so the calls of a test fall into one fixed rate window.
func freshMinute() {
	if s := time.Now().Second(); s > 40 {
		time.Sleep(time.Duration(61-s) * time.Second)
	}
}

// polled reports whether a poll of userCode reached the storage.
func polled(t *testing.T, s stand, userCode string) bool {
	t.Helper()
	var ok bool
	require.NoError(t, s.pool.QueryRow(context.Background(),
		"SELECT last_poll_at IS NOT NULL FROM oauth_device_code WHERE user_code = $1", userCode).Scan(&ok))
	return ok
}

// TestTokenGuard: POST /oauth/token counts per IP; token exchange requests
// share the count and a device flow works under the limit; past TokenPerIP
// in a minute the answer is 429 with Retry-After 60 and the request never
// reaches the storage. Discovery and the device endpoint are not limited.
func TestTokenGuard(t *testing.T) {
	s := newStand(t)
	freshMinute()
	dr := startDevice(t, s.srv)
	require.NoError(t, s.p.ApproveDeviceCode(context.Background(), cookieAs(s.fx, s.fx.MemberID), dr.UserCode))
	exchange := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:token-exchange"}, "client_id": {oauth.ClientCLI},
		"subject_token": {"x"}, "subject_token_type": {"urn:ietf:params:oauth:token-type:access_token"}}
	for i := range oauth.TokenPerIP - 2 {
		er := postToken(t, s, exchange, "")
		require.Equal(t, http.StatusBadRequest, er.Status, "request %d: %s", i+1, er.Error)
	}
	tr := poll(t, s.srv, dr.DeviceCode)
	require.Equal(t, http.StatusOK, tr.Status, tr.Error)
	er := postToken(t, s, exchange, "")
	require.Equal(t, http.StatusBadRequest, er.Status, "the last request within the limit")

	next := startDevice(t, s.srv)
	res, err := http.PostForm(s.srv.URL+"/oauth/token", url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"},
		"client_id": {oauth.ClientCLI}, "device_code": {next.DeviceCode}})
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	var body map[string]string
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, http.StatusTooManyRequests, res.StatusCode)
	assert.Equal(t, "60", res.Header.Get("Retry-After"))
	assert.Equal(t, map[string]string{"error": "too_many_requests"}, body)
	assert.False(t, polled(t, s, next.UserCode), "the refused request never reaches the storage")
	_, _ = getJSON(t, s.srv.URL+"/.well-known/openid-configuration")
}

// TestTokenLimitStoreErrorDenies: when the rate store fails, the token
// endpoint refuses (fail-closed) before the storage.
func TestTokenLimitStoreErrorDenies(t *testing.T) {
	s := newStand(t)
	dr := startDevice(t, s.srv)
	_, err := s.pool.Exec(context.Background(), "DROP TABLE rate_limit")
	require.NoError(t, err)
	tr := poll(t, s.srv, dr.DeviceCode)
	assert.Equal(t, http.StatusInternalServerError, tr.Status)
	assert.Equal(t, "server_error", tr.Error)
	assert.False(t, polled(t, s, dr.UserCode))
}

// TestDevicePollSlowDown: a pending code polled again within
// DevicePollInterval gets slow_down (RFC 8628 §3.5); after the interval it is
// pending again. An approved code answers with tokens at once.
func TestDevicePollSlowDown(t *testing.T) {
	s := newStand(t)
	dr := startDevice(t, s.srv)
	assert.Equal(t, "authorization_pending", poll(t, s.srv, dr.DeviceCode).Error)
	tr := poll(t, s.srv, dr.DeviceCode)
	assert.Equal(t, http.StatusBadRequest, tr.Status)
	assert.Equal(t, "slow_down", tr.Error)

	_, err := s.pool.Exec(context.Background(), "UPDATE oauth_device_code SET last_poll_at = now() - interval '6 seconds' WHERE user_code = $1", dr.UserCode)
	require.NoError(t, err)
	assert.Equal(t, "authorization_pending", poll(t, s.srv, dr.DeviceCode).Error)

	require.NoError(t, s.p.ApproveDeviceCode(context.Background(), cookieAs(s.fx, s.fx.MemberID), dr.UserCode))
	tr = poll(t, s.srv, dr.DeviceCode)
	assert.Equal(t, http.StatusOK, tr.Status, tr.Error)
	assert.Equal(t, 5*time.Second, oauth.DevicePollInterval)
}
