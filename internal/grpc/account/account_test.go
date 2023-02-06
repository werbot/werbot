package account_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	accountpb "github.com/werbot/werbot/api/proto/account"
	"github.com/werbot/werbot/internal/utils/test"
)

func Test_account_AccountIDByLogin(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	postgres := test.CreateDB(t, "../../../migration", "../../../fixtures/migration")
	defer postgres.Stop(t)

	grpc := test.CreateGRPC(ctx, t, &test.Service{DB: postgres.Conn})
	defer grpc.Close()

	testCases := []struct {
		name    string
		req     *accountpb.AccountIDByLogin_Request
		resp    *accountpb.AccountIDByLogin_Response
		respErr string
	}{
		{
			name: "test",
			req: &accountpb.AccountIDByLogin_Request{
				Login:       "admin",
				Fingerprint: "b6:07:6a:ef:82:e3:73:47:56:69:3f:3d:c7:d7:6f:23",
			},
			resp: &accountpb.AccountIDByLogin_Response{
				UserId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
			},
		},
		{
			name: "test2",
			req: &accountpb.AccountIDByLogin_Request{
				Login:       "user",
				Fingerprint: "b6:07:6a:ef:82:e3:73:47:56:69:3f:3d:c7:d7:6f:23",
			},
			resp:    &accountpb.AccountIDByLogin_Response{},
			respErr: "rpc error: code = Unknown desc = not found",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			a := accountpb.NewAccountHandlersClient(grpc)
			response, err := a.AccountIDByLogin(ctx, tt.req)
			if err != nil {
				require.EqualError(t, err, tt.respErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.resp.UserId, response.UserId)
		})
	}

}
