package oauth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/identity/oauth"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/secretbox"
)

const testWebURL = "https://ui.werbot.test"

func testBox(t *testing.T) *secretbox.Box {
	t.Helper()
	box, err := secretbox.New(bytes.Repeat([]byte{7}, 32), nil)
	require.NoError(t, err)
	return box
}

// newTestProvider serves a provider on a loopback listener; its URL is the issuer.
func newTestProvider(t *testing.T) (*oauth.SigningKey, *pgxpool.Pool, *httptest.Server) {
	t.Helper()
	pool, _ := dbtest.New(t)
	key, err := oauth.LoadSigningKey(context.Background(), pool, testBox(t))
	require.NoError(t, err)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	p, err := oauth.New(pool, key, nil, oauth.Config{Issuer: "http://" + ln.Addr().String(), WebURL: testWebURL}, slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	srv := &httptest.Server{Listener: ln, Config: &http.Server{Handler: p.Handler()}} //nolint:gosec // test server
	srv.Start()
	t.Cleanup(srv.Close)
	return key, pool, srv
}

// noRedirect does not follow redirects, so tests see each Location.
var noRedirect = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func getJSON(t *testing.T, url string) (body []byte, doc map[string]any) {
	t.Helper()
	res, err := http.Get(url) //nolint:gosec // loopback test server
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusOK, res.StatusCode, url)
	require.NoError(t, json.NewDecoder(res.Body).Decode(&doc))
	body, err = json.Marshal(doc)
	require.NoError(t, err)
	return body, doc
}
