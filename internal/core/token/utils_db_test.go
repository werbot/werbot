package token_test

import (
	"context"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stretchr/testify/assert"
	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	tokenpb "github.com/werbot/werbot/internal/core/token/proto/rpc"
	"github.com/werbot/werbot/internal/utils/test"
)

// Test_FindActiveTokenByProfileAndAction_ExpiredAt tests expired_at validation
// This function is used internally by AddTokenProfileReset, so we test it indirectly
func Test_FindActiveTokenByProfileAndAction_ExpiredAt(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	client := tokenpb.NewTokenHandlersClient(setup)
	ctx := context.Background()
	profileID := test.ConstUserID
	now := time.Now()

	// Create token with custom expiration (1 hour from now)
	resp1, err := client.AddTokenProfileReset(ctx, &tokenmessage.AddTokenProfileReset_Request{
		ProfileId: profileID,
		ExpiredAt: timestamppb.New(now.Add(1 * time.Hour)),
	})
	assert.NoError(t, err)
	token1 := resp1.GetToken()
	assert.NotEmpty(t, token1)

	// Verify token can be retrieved (not expired yet)
	tokenData1, err := client.Token(ctx, &tokenmessage.Token_Request{
		Token: token1,
	})
	assert.NoError(t, err)
	assert.Equal(t, tokenenum.Action_reset, tokenData1.GetAction())

	// Create token with expired_at in the past
	resp2, err := client.AddTokenProfileReset(ctx, &tokenmessage.AddTokenProfileReset_Request{
		ProfileId: profileID,
		ExpiredAt: timestamppb.New(now.Add(-1 * time.Hour)),
	})
	assert.NoError(t, err)
	token2 := resp2.GetToken()
	assert.NotEmpty(t, token2)

	// Token with expired_at in past should return error when accessed
	_, err = client.Token(ctx, &tokenmessage.Token_Request{
		Token: token2,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}
