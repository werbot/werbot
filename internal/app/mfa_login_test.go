package app

import (
	"context"
	"log/slog"
	"net/http"
	"testing"
	"testing/fstest"
	"time"

	"connectrpc.com/connect"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

func TestServeLoginMFAFlow(t *testing.T) {
	pool, _ := dbtest.New(t)
	cfg := config.Config{HTTPAddr: freeAddr(t), Env: "development", WebEmbedded: true}
	startServe(t, cfg, pool, nil, fstest.MapFS{}, slog.New(slog.DiscardHandler))
	hc, _ := jarClient(t, http.DefaultTransport)
	auth := werbotv1connect.NewAuthServiceClient(hc, "http://"+cfg.HTTPAddr)
	mfa := werbotv1connect.NewMFAServiceClient(hc, "http://"+cfg.HTTPAddr)
	ctx := context.Background()

	_, err := auth.Setup(ctx, connect.NewRequest(ownerSetup))
	require.NoError(t, err)
	enr, err := mfa.EnrollTOTP(ctx, connect.NewRequest(&werbotv1.EnrollTOTPRequest{}))
	require.NoError(t, err)
	code := func(step int64) string {
		c, err := totp.GenerateCodeCustom(enr.Msg.GetSecret(), time.Unix(step*30, 0),
			totp.ValidateOpts{Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
		require.NoError(t, err)
		return c
	}
	step := time.Now().Unix() / 30
	_, err = mfa.ConfirmTOTP(ctx, connect.NewRequest(&werbotv1.ConfirmTOTPRequest{Code: code(step)}))
	require.NoError(t, err)
	_, err = auth.Logout(ctx, connect.NewRequest(&werbotv1.LogoutRequest{}))
	require.NoError(t, err)

	res, err := auth.Login(ctx, connect.NewRequest(&werbotv1.LoginRequest{Email: ownerSetup.GetEmail(), Password: ownerPassword}))
	require.NoError(t, err)
	assert.True(t, res.Msg.GetMfaRequired())
	assert.Equal(t, []string{"totp"}, res.Msg.GetMethods())
	assert.Empty(t, res.Header().Get("Set-Cookie"), "no session before the second factor")
	_, err = auth.GetSession(ctx, connect.NewRequest(&werbotv1.GetSessionRequest{}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))

	_, err = auth.CompleteMFA(ctx, connect.NewRequest(&werbotv1.CompleteMFARequest{MfaToken: res.Msg.GetMfaToken()}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "one proof is required")
	done, err := auth.CompleteMFA(ctx, connect.NewRequest(&werbotv1.CompleteMFARequest{
		MfaToken: res.Msg.GetMfaToken(), Proof: &werbotv1.CompleteMFARequest_TotpCode{TotpCode: code(step + 1)},
	}))
	require.NoError(t, err, "CompleteMFA is public")
	c, err := http.ParseSetCookie(done.Header().Get("Set-Cookie"))
	require.NoError(t, err)
	assert.Equal(t, "werbot_session", c.Name)
	sess, err := auth.GetSession(ctx, connect.NewRequest(&werbotv1.GetSessionRequest{}))
	require.NoError(t, err)
	assert.Equal(t, ownerSetup.GetEmail(), sess.Msg.GetUser().GetEmail())
	assert.False(t, sess.Msg.GetMfaEnrollmentRequired())
}
