package app

import (
	"context"
	"log/slog"
	"net/http"
	"testing"
	"testing/fstest"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/identity"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/db"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// TestLoginRateLimitAcrossReplicas: two server replicas on one database,
// behind a trusted proxy, share the login count of one client IP and email:
// past the limit either replica refuses even the right password.
func TestLoginRateLimitAcrossReplicas(t *testing.T) {
	ctx := context.Background()
	poolA, url := dbtest.New(t)
	poolB, err := db.Open(ctx, url)
	require.NoError(t, err)
	t.Cleanup(poolB.Close)
	client := &http.Client{Transport: headerTransport{"X-Forwarded-For", "198.51.100.9"}}
	var replicas []werbotv1connect.AuthServiceClient
	for _, pool := range []*pgxpool.Pool{poolA, poolB} {
		cfg := config.Config{HTTPAddr: freeAddr(t), Env: "development", WebEmbedded: true, TrustedProxies: loopback}
		startServe(t, cfg, pool, nil, fstest.MapFS{}, slog.New(slog.DiscardHandler))
		replicas = append(replicas, werbotv1connect.NewAuthServiceClient(client, "http://"+cfg.HTTPAddr))
	}
	_, err = replicas[0].Setup(ctx, connect.NewRequest(ownerSetup))
	require.NoError(t, err)

	dbtest.FreshRateWindow() // one fixed rate window for every attempt
	wrong := &werbotv1.LoginRequest{Email: ownerSetup.GetEmail(), Password: "wrong horse battery"}
	for i := range identity.LoginPerIPEmail {
		_, err := replicas[i%2].Login(ctx, connect.NewRequest(wrong))
		require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err), "attempt %d", i+1)
	}
	right := &werbotv1.LoginRequest{Email: ownerSetup.GetEmail(), Password: ownerPassword}
	for _, r := range replicas {
		_, err := r.Login(ctx, connect.NewRequest(right))
		var ce *connect.Error
		require.ErrorAs(t, err, &ce)
		assert.Equal(t, connect.CodeResourceExhausted, ce.Code())
		assert.Equal(t, "too many requests", ce.Message())
	}
}
