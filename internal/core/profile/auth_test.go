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
)

func Test_ResetPasswordRoundTrip(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	ctx := context.Background()
	profileClient := profilerpc.NewProfileHandlersClient(setup)
	tokenClient := tokenpb.NewTokenHandlersClient(setup)

	reset, err := tokenClient.AddTokenProfileReset(ctx, &tokenmessage.AddTokenProfileReset_Request{
		ProfileId: test.ConstUserID,
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
		Email:    test.ConstUserEmail,
		Password: test.ConstUserPassword,
	})
	require.Error(t, err, "the old password must stop working")

	profile, err := profileClient.SignIn(ctx, &profilemessage.SignIn_Request{
		Email:    test.ConstUserEmail,
		Password: newPassword,
	})
	require.NoError(t, err)
	assert.Equal(t, test.ConstUserID, profile.GetProfileId())
}
