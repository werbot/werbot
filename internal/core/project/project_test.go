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
	setup, teardownTestCase := test.GRPC(t)
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

func Test_ProjectCounts(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	builder := factory.New(t, setup.Postgres.Conn().Conn)
	admin := builder.Profile(factory.ProfileParams{Alias: "admin", Role: 3})
	memberOne := builder.Profile(factory.ProfileParams{Alias: "member1", Role: 1})
	memberTwo := builder.Profile(factory.ProfileParams{Alias: "member2", Role: 1})

	project := builder.Project(factory.ProjectParams{
		OwnerID: admin.ID,
		Title:   "project-with-schemes",
	})
	builder.ProjectMember(factory.ProjectMemberParams{ProjectID: project.ID, ProfileID: memberOne.ID})
	builder.ProjectMember(factory.ProjectMemberParams{ProjectID: project.ID, ProfileID: memberTwo.ID})

	// Different count per group and scheme subtype so a wrong column
	// mapping or exact-type match fails the test.
	for schemeType, count := range map[int16]int{
		101: 2,
		201: 3,
		601: 4,
		301: 5,
		401: 6,
		501: 7,
	} {
		for range count {
			builder.Scheme(factory.SchemeParams{ProjectID: project.ID, SchemeType: schemeType})
		}
	}

	counts := test.BodyTable{
		"servers_count":      float64(2),
		"databases_count":    float64(3),
		"applications_count": float64(4),
		"desktops_count":     float64(5),
		"containers_count":   float64(6),
		"clouds_count":       float64(7),
		"members_count":      float64(2),
	}

	projectHandler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := projectpb.NewProjectHandlersClient(setup)
		return a.Project(ctx, req.(*projectmessage.Project_Request))
	}
	test.RunCaseGRPCTests(t, projectHandler, test.GRPCTable{
		Name: "counts group scheme subtypes and ignore the member cross join",
		Request: &projectmessage.Project_Request{
			IsAdmin:   true,
			OwnerId:   admin.ID,
			ProjectId: project.ID,
		},
		Response: counts,
	})

	projectsHandler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := projectpb.NewProjectHandlersClient(setup)
		return a.Projects(ctx, req.(*projectmessage.Projects_Request))
	}
	listResponse := test.BodyTable{"total": float64(1)}
	for key, value := range counts {
		listResponse["projects.0."+key] = value
	}
	test.RunCaseGRPCTests(t, projectsHandler, test.GRPCTable{
		Name: "list counts group scheme subtypes and ignore the member cross join",
		Request: &projectmessage.Projects_Request{
			IsAdmin: true,
			OwnerId: admin.ID,
		},
		Response: listResponse,
	})
}
