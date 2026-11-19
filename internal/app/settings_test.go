package app

import (
	"context"
	"log/slog"
	"testing"
	"testing/fstest"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// TestServeSettingsAfterSetup: Setup creates the settings row and serve mounts
// SettingsService behind the request validation.
func TestServeSettingsAfterSetup(t *testing.T) {
	pool, _ := dbtest.New(t)
	cfg := config.Config{HTTPAddr: freeAddr(t), Env: "development", WebEmbedded: true}
	startServe(t, cfg, pool, nil, fstest.MapFS{}, slog.New(slog.DiscardHandler))
	hc, _ := jarClient(t, nil)
	base := "http://" + cfg.HTTPAddr
	ctx := context.Background()
	_, err := werbotv1connect.NewAuthServiceClient(hc, base).Setup(ctx, connect.NewRequest(ownerSetup))
	require.NoError(t, err)
	settings := werbotv1connect.NewSettingsServiceClient(hc, base)

	got, err := settings.GetSettings(ctx, connect.NewRequest(&werbotv1.GetSettingsRequest{}))
	require.NoError(t, err)
	assert.Equal(t, 168*time.Hour, got.Msg.GetSettings().GetCliSessionTtl().AsDuration())
	assert.Equal(t, 720*time.Hour, got.Msg.GetSettings().GetCliSessionMax().AsDuration())

	_, err = settings.UpdateSettings(ctx, connect.NewRequest(&werbotv1.UpdateSettingsRequest{
		Settings:   &werbotv1.OrgSettings{CliSessionTtl: durationpb.New(30 * time.Minute)},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"cli_session_ttl"}},
	}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "protovalidate interceptor")
}
