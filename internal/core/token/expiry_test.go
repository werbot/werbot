package token_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	tokenpb "github.com/werbot/werbot/internal/core/token/proto/rpc"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/internal/utils/test/factory"
)

func Test_TokenExpiry(t *testing.T) {
	pgTest := test.ServerPostgres(t, "../../../migration")
	redisTest := test.ServerRedis(context.Background(), t)
	grpcTest, err := test.ServerGRPC(context.Background(), t, pgTest, redisTest)
	require.NoError(t, err)
	defer func() {
		grpcTest.Close()
		redisTest.Close()
	}()

	ctx := context.Background()
	client := tokenpb.NewTokenHandlersClient(grpcTest)

	builder := factory.New(t, pgTest.Conn().Conn)
	user := builder.Profile(factory.ProfileParams{Alias: "user", Name: "user", Surname: "user", Email: "user@werbot.net"})

	oldToken := builder.Token(factory.TokenParams{
		Section:   int16(tokenenum.Section_profile),
		Action:    int16(tokenenum.Action_reset),
		Status:    int16(tokenenum.Status_sent),
		ProfileID: user.ID,
		CreatedAt: factory.Time(time.Now().UTC().Add(-48 * time.Hour)),
	})

	_, err = client.Token(ctx, &tokenmessage.Token_Request{Token: oldToken.ID})
	require.Error(t, err, "a token with a NULL expired_at older than 24 hours must be rejected")
	assert.Contains(t, err.Error(), "expired")

	reset, err := client.AddTokenProfileReset(ctx, &tokenmessage.AddTokenProfileReset_Request{
		ProfileId: user.ID,
	})
	require.NoError(t, err)

	var hasDefaultExpiry bool
	err = pgTest.Conn().Conn.QueryRowContext(ctx, `
    SELECT
      "expired_at" IS NOT NULL
      AND "expired_at" BETWEEN CURRENT_TIMESTAMP + INTERVAL '23 hour' AND CURRENT_TIMESTAMP + INTERVAL '25 hour'
    FROM "token" WHERE "id" = $1
  `, reset.GetToken()).Scan(&hasDefaultExpiry)
	require.NoError(t, err)
	assert.True(t, hasDefaultExpiry, "reset token must get a default expiration of 24 hours")

	tokenData, err := client.Token(ctx, &tokenmessage.Token_Request{Token: reset.GetToken()})
	require.NoError(t, err)
	assert.Equal(t, tokenenum.Action_reset, tokenData.GetAction())
}
