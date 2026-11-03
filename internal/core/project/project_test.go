package project_test

import (
	"context"
	"testing"

	projectmessage "github.com/werbot/werbot/internal/core/project/proto/message"
	projectpb "github.com/werbot/werbot/internal/core/project/proto/rpc"
	"github.com/werbot/werbot/internal/utils/test"
	"google.golang.org/grpc/codes"
)

func Test_Project(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := projectpb.NewProjectHandlersClient(setup)
		return a.Project(ctx, req.(*projectmessage.Project_Request))
	}

	testTable := []test.GRPCTable{
		{ // request without parameters
			Name:    "test0_01",
			Request: &projectmessage.Project_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id":   "value is required",
					"project_id": "value is required",
				},
			},
		},
		{ // ADMIN: response carries the requested ids
			Name: "test1_01",
			Request: &projectmessage.Project_Request{
				IsAdmin:   true,
				OwnerId:   test.ConstAdminID,
				ProjectId: test.ConstAdminProject1ID,
			},
			Response: test.BodyTable{
				"project_id": test.ConstAdminProject1ID,
				"owner_id":   test.ConstAdminID,
				"title":      "project1",
				"created_at": "*",
			},
		},
		{ // USER: owner is hidden but the project id is still returned
			Name: "test2_01",
			Request: &projectmessage.Project_Request{
				OwnerId:   test.ConstUserID,
				ProjectId: test.ConstUserProject1ID,
			},
			Response: test.BodyTable{
				"project_id": test.ConstUserProject1ID,
				"owner_id":   nil,
				"title":      "project3",
				"created_at": "*",
			},
		},
		{ // ADMIN: unknown project
			Name: "test1_02",
			Request: &projectmessage.Project_Request{
				IsAdmin:   true,
				OwnerId:   test.ConstAdminID,
				ProjectId: test.ConstFakeID,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Not found",
			},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			test.RunCaseGRPCTests(t, handler, tt)
		})
	}
}
