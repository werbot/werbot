package profile_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	profilemessage "github.com/werbot/werbot/internal/core/profile/proto/message"
	profilerpc "github.com/werbot/werbot/internal/core/profile/proto/rpc"
	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	tokenpb "github.com/werbot/werbot/internal/core/token/proto/rpc"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/internal/utils/test/factory"
)

const (
	authUserEmail    = "user@werbot.net"
	authUserPassword = "user@werbot.net"
)

func Test_ResetPasswordRoundTrip(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	builder := factory.New(t, setup.Postgres.Conn().Conn)
	user := builder.Profile(factory.ProfileParams{
		Alias:    "user",
		Email:    authUserEmail,
		Password: authUserPassword,
		Role:     1,
	})

	ctx := context.Background()
	profileClient := profilerpc.NewProfileHandlersClient(setup)
	tokenClient := tokenpb.NewTokenHandlersClient(setup)

	reset, err := tokenClient.AddTokenProfileReset(ctx, &tokenmessage.AddTokenProfileReset_Request{
		ProfileId: user.ID,
	})
	require.NoError(t, err)
	require.NotEmpty(t, reset.GetToken())

	const newPassword = "NewSecurePass1"
	_, err = profileClient.ResetPassword(ctx, &profilemessage.ResetPassword_Request{
		Request: &profilemessage.ResetPassword_Request_Password{
			Password: &profilemessage.ResetPassword_Password{
				Password: newPassword,
				Token:    reset.GetToken(),
			},
		},
	})
	require.NoError(t, err)

	_, err = profileClient.SignIn(ctx, &profilemessage.SignIn_Request{
		Email:    user.Email,
		Password: authUserPassword,
	})
	require.Error(t, err, "the old password must stop working")

	profile, err := profileClient.SignIn(ctx, &profilemessage.SignIn_Request{
		Email:    user.Email,
		Password: newPassword,
	})
	require.NoError(t, err)
	assert.Equal(t, user.ID, profile.GetProfileId())
}
