package profile_test

import (
	"context"
	"testing"

	profilemessage "github.com/werbot/werbot/internal/core/profile/proto/message"
	profilerpc "github.com/werbot/werbot/internal/core/profile/proto/rpc"
	"github.com/werbot/werbot/internal/utils/test"
	"google.golang.org/grpc/codes"
)

func Test_Profiles(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := profilerpc.NewProfileHandlersClient(setup)
		return a.Profiles(ctx, req.(*profilemessage.Profiles_Request))
	}

	testTable := []test.GRPCTable{
		{ // list items carry created_at
			Name: "test1_01",
			Request: &profilemessage.Profiles_Request{
				IsAdmin: true,
				Limit:   2,
			},
			Response: test.BodyTable{
				"profiles.0":            "*",
				"profiles.1":            "*",
				"profiles.2":            nil,
				"profiles.0.created_at": "*",
				"profiles.1.created_at": "*",
			},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			test.RunCaseGRPCTests(t, handler, tt)
		})
	}
}

func Test_ProfileIDByLogin(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := profilerpc.NewProfileHandlersClient(setup)
		return a.ProfileIDByLogin(ctx, req.(*profilemessage.ProfileIDByLogin_Request))
	}

	testTable := []test.GRPCTable{
		{
			Name: "test0_01",
			Request: &profilemessage.ProfileIDByLogin_Request{
				Login:       "admin",
				Fingerprint: "b6:07:6a:ef:82:e3:73:47:56:69:3f:3d:c7:d7:6f:23",
			},
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
