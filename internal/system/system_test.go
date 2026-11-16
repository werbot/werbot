package system_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/version"
	"github.com/werbot/werbot/internal/system"
)

func TestGetVersion(t *testing.T) {
	version.Version, version.Commit, version.BuildDate = "1.2.3", "abc", "2026-11-06"
	mux := http.NewServeMux()
	mux.Handle(werbotv1connect.NewSystemServiceHandler(system.New()))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	res, err := werbotv1connect.NewSystemServiceClient(srv.Client(), srv.URL).
		GetVersion(context.Background(), connect.NewRequest(&werbotv1.GetVersionRequest{}))
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", res.Msg.GetVersion())
	assert.Equal(t, "abc", res.Msg.GetCommit())
	assert.Equal(t, "2026-11-06", res.Msg.GetBuildDate())
}

func newClient(t *testing.T) werbotv1connect.SystemServiceClient {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(werbotv1connect.NewSystemServiceHandler(system.New()))
	srv := httptest.NewServer(clientip.Middleware(nil)(mux))
	t.Cleanup(srv.Close)
	return werbotv1connect.NewSystemServiceClient(srv.Client(), srv.URL)
}

func TestListCountries(t *testing.T) {
	res, err := newClient(t).ListCountries(context.Background(), connect.NewRequest(&werbotv1.ListCountriesRequest{}))
	require.NoError(t, err)
	cs := res.Msg.GetCountries()
	assert.Len(t, cs, 249)
	seen := map[string]bool{}
	for _, c := range cs {
		assert.Regexp(t, `^[A-Z]{2}$`, c.GetCode())
		assert.False(t, seen[c.GetCode()], c.GetCode())
		seen[c.GetCode()] = true
	}
	assert.Contains(t, cs, &werbotv1.Country{Code: "DE", Name: "Germany"})
	assert.Contains(t, cs, &werbotv1.Country{Code: "CI", Name: "Côte d'Ivoire"})
}

func TestGetClientIP(t *testing.T) {
	res, err := newClient(t).GetClientIP(context.Background(), connect.NewRequest(&werbotv1.GetClientIPRequest{}))
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", res.Msg.GetIp())
}
