package account_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	accountpb "github.com/werbot/werbot/internal/grpc/account/proto"
	"github.com/werbot/werbot/internal/utils/test"
)

type testSetup struct {
	ctx  context.Context
	grpc *grpc.ClientConn
}

func setupTest(t *testing.T) (testSetup, func(t *testing.T)) {
	ctx := context.Background()

	postgres, err := test.Postgres(t, "../../../migration", "../../../fixtures/migration")
	grpc, _ := test.GRPC(ctx, t, postgres.Conn, nil)

	if err != nil {
		t.Error(err)
	}

	return testSetup{
			ctx:  ctx,
			grpc: grpc.ClientConn,
		}, func(t *testing.T) {
			postgres.Close()
			grpc.Close()
		}
}

func Test_account_AccountIDByLogin(t *testing.T) {
	//t.Parallel()
	setup, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

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
			respErr: "rpc error: code = NotFound desc = Not found",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			a := accountpb.NewAccountHandlersClient(setup.grpc)
			response, err := a.AccountIDByLogin(setup.ctx, tt.req)
			if err != nil {
				assert.EqualError(t, err, tt.respErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.resp.UserId, response.UserId)
		})
	}

}
