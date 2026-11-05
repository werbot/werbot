package project_test

import (
	"context"
	"testing"

	projectmessage "github.com/werbot/werbot/internal/core/project/proto/message"
	projectpb "github.com/werbot/werbot/internal/core/project/proto/rpc"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/internal/utils/test/factory"
	"github.com/werbot/werbot/pkg/uuid"
	"google.golang.org/grpc/codes"
)

func Test_Project(t *testing.T) {
	setup, teardownTestCase := test.GRPCWithoutFixtures(t)
	defer teardownTestCase(t)

	builder := factory.New(t, setup.Postgres.Conn().Conn)
	admin := builder.Profile(factory.ProfileParams{Alias: "admin", Role: 3})
	user := builder.Profile(factory.ProfileParams{Alias: "user", Role: 1})

	adminProject := builder.Project(factory.ProjectParams{
		OwnerID: admin.ID,
		Title:   "project1",
	})
	userProject := builder.Project(factory.ProjectParams{
		OwnerID: user.ID,
		Title:   "project3",
	})
	unknownProjectID := uuid.New()

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
				OwnerId:   admin.ID,
				ProjectId: adminProject.ID,
			},
			Response: test.BodyTable{
				"project_id": adminProject.ID,
				"owner_id":   admin.ID,
				"title":      "project1",
				"created_at": "*",
			},
		},
		{ // USER: owner is hidden but the project id is still returned
			Name: "test2_01",
			Request: &projectmessage.Project_Request{
				OwnerId:   user.ID,
				ProjectId: userProject.ID,
			},
			Response: test.BodyTable{
				"project_id": userProject.ID,
				"owner_id":   nil,
				"title":      "project3",
				"created_at": "*",
			},
		},
		{ // ADMIN: unknown project
			Name: "test1_02",
			Request: &projectmessage.Project_Request{
				IsAdmin:   true,
				OwnerId:   admin.ID,
				ProjectId: unknownProjectID,
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
