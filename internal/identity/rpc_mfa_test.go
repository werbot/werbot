package identity

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
)

// newMFAClient serves AuthService and MFAService behind Middleware with the
// validation interceptor, as serve does, and returns alice's session cookie.
func newMFAClient(t *testing.T, svc *Service) (werbotv1connect.MFAServiceClient, *http.Cookie) {
	t.Helper()
	h := NewHandler(svc)
	opts := connect.WithInterceptors(validate.NewInterceptor())
	mux := http.NewServeMux()
	mux.Handle(werbotv1connect.NewAuthServiceHandler(h, opts))
	mux.Handle(werbotv1connect.NewMFAServiceHandler(h, opts))
	srv := httptest.NewServer(Middleware(svc, nil)(mux))
	t.Cleanup(srv.Close)
	c := login(t, werbotv1connect.NewAuthServiceClient(srv.Client(), srv.URL))
	return werbotv1connect.NewMFAServiceClient(srv.Client(), srv.URL), c
}

func TestMFAServiceRPC(t *testing.T) {
	svc, _, _ := newMFATest(t)
	client, c := newMFAClient(t, svc)
	ctx := context.Background()

	_, err := client.ListFactors(ctx, connect.NewRequest(&werbotv1.ListFactorsRequest{}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	_, err = client.ConfirmTOTP(ctx, withCookie(&werbotv1.ConfirmTOTPRequest{Code: "12345"}, c.Name, c.Value))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	_, err = client.DeleteFactor(ctx, withCookie(&werbotv1.DeleteFactorRequest{Id: "sms"}, c.Name, c.Value))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	enr, err := client.EnrollTOTP(ctx, withCookie(&werbotv1.EnrollTOTPRequest{}, c.Name, c.Value))
	require.NoError(t, err)
	assert.NotEmpty(t, enr.Msg.GetOtpauthUrl())
	assert.Len(t, enr.Msg.GetSecret(), 32)
	assert.Positive(t, enr.Msg.GetQrSize())
	js, err := protojson.Marshal(enr.Msg)
	require.NoError(t, err)
	assert.NotContains(t, string(js), "ciphertext")
	assert.NotContains(t, string(js), "keyId")

	_, err = client.ConfirmTOTP(ctx, withCookie(&werbotv1.ConfirmTOTPRequest{Code: codeAt(t, enr.Msg.GetSecret(), currentStep())}, c.Name, c.Value))
	require.NoError(t, err)
	_, err = client.EnrollTOTP(ctx, withCookie(&werbotv1.EnrollTOTPRequest{}, c.Name, c.Value))
	assert.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err))

	codes, err := client.GenerateRecoveryCodes(ctx, withCookie(&werbotv1.GenerateRecoveryCodesRequest{}, c.Name, c.Value))
	require.NoError(t, err)
	assert.Len(t, codes.Msg.GetCodes(), RecoveryCodeCount)

	list, err := client.ListFactors(ctx, withCookie(&werbotv1.ListFactorsRequest{}, c.Name, c.Value))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetFactors(), 2)
	assert.Equal(t, "totp", list.Msg.GetFactors()[0].GetKind())
	assert.NotNil(t, list.Msg.GetFactors()[0].GetCreatedAt())
	assert.Equal(t, int32(RecoveryCodeCount), list.Msg.GetFactors()[1].GetRemaining())

	_, err = client.DeleteFactor(ctx, withCookie(&werbotv1.DeleteFactorRequest{Id: "recovery"}, c.Name, c.Value))
	require.NoError(t, err)
	_, err = client.DeleteFactor(ctx, withCookie(&werbotv1.DeleteFactorRequest{Id: "recovery"}, c.Name, c.Value))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// TestNoMFASecretsInLogs: the TOTP secret, its otpauth URL and the recovery
// codes appear only in the EnrollTOTP and GenerateRecoveryCodes responses:
// never in logs or audit rows, on success or failure.
func TestNoMFASecretsInLogs(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	prev := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(prev) })
	svc, pool, p := newMFATest(t)
	svc.cfg.Logger = logger
	client, c := newMFAClient(t, svc)
	ctx := context.Background()

	enr, err := client.EnrollTOTP(ctx, withCookie(&werbotv1.EnrollTOTPRequest{}, c.Name, c.Value))
	require.NoError(t, err)
	secret := enr.Msg.GetSecret()
	step := currentStep()
	_, err = client.ConfirmTOTP(ctx, withCookie(&werbotv1.ConfirmTOTPRequest{Code: wrongCode(t, secret, step)}, c.Name, c.Value))
	require.Error(t, err)
	good := codeAt(t, secret, step)
	_, err = client.ConfirmTOTP(ctx, withCookie(&werbotv1.ConfirmTOTPRequest{Code: good}, c.Name, c.Value))
	require.NoError(t, err)
	codes, err := client.GenerateRecoveryCodes(ctx, withCookie(&werbotv1.GenerateRecoveryCodesRequest{}, c.Name, c.Value))
	require.NoError(t, err)
	require.NoError(t, recoverInTx(svc, p, codes.Msg.GetCodes()[0]))
	require.ErrorIs(t, recoverInTx(svc, p, codes.Msg.GetCodes()[0]), ErrInvalidCode)

	var audit string
	require.NoError(t, pool.QueryRow(ctx, "SELECT coalesce(string_agg(data::text || action, ' '), '') FROM audit_event").Scan(&audit))
	require.Contains(t, audit, "mfa.factor.add")
	secrets := append([]string{secret, enr.Msg.GetOtpauthUrl(), good}, codes.Msg.GetCodes()...)
	for _, s := range secrets {
		assert.NotContains(t, logs.String(), s)
		assert.NotContains(t, audit, s)
	}
}
