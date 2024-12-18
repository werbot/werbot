package profile_test

import (
	"context"
	"testing"

	profilepb "github.com/werbot/werbot/internal/core/profile/proto/profile"
	"github.com/werbot/werbot/internal/utils/test"
	"google.golang.org/grpc/codes"
)

func Test_ProfileIDByLogin(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := profilepb.NewProfileHandlersClient(setup)
		return a.ProfileIDByLogin(ctx, req.(*profilepb.ProfileIDByLogin_Request))
	}

	testTable := []test.GRPCTable{
		{
			Name: "test0_01",
			Request: &profilepb.ProfileIDByLogin_Request{
				Login:       "admin",
				Fingerprint: "b6:07:6a:ef:82:e3:73:47:56:69:3f:3d:c7:d7:6f:23",
			},
			//Response: &accountpb.AccountIDByLogin_Response{
			//	UserId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
			//},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"client_ip": "value is empty, which is not a valid IP address",
				},
			},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			test.RunCaseGRPCTests(t, handler, tt)
		})
	}
}
