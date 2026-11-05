package scheme_test

import (
	"context"
	"fmt"
	"testing"

	schemeaccesspb "github.com/werbot/werbot/internal/core/scheme/proto/access"
	schemepb "github.com/werbot/werbot/internal/core/scheme/proto/scheme"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/internal/utils/test/factory"
	"google.golang.org/grpc/codes"
)

// schemeAccess returns access data with the given alias and no auth markers.
func schemeAccess(alias string) map[string]any {
	return map[string]any{
		"alias":   alias,
		"address": "127.0.0.1",
		"port":    22,
	}
}

func Test_ProfileSchemes(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	builder := factory.New(t, setup.Postgres.Conn().Conn)

	user := builder.Profile(factory.ProfileParams{Alias: "user"})
	admin := builder.Profile(factory.ProfileParams{Alias: "admin", Role: 3})
	user1 := builder.Profile(factory.ProfileParams{Alias: "user1"})

	userProject := builder.Project(factory.ProjectParams{
		OwnerID: user.ID,
		Alias:   "userproj",
		Title:   "user project",
	})
	sharedProject := builder.Project(factory.ProjectParams{
		OwnerID: admin.ID,
		Alias:   "shared",
		Title:   "shared project",
	})

	userMember := builder.ProjectMember(factory.ProjectMemberParams{ProjectID: userProject.ID, ProfileID: user.ID})
	sharedUserMember := builder.ProjectMember(factory.ProjectMemberParams{ProjectID: sharedProject.ID, ProfileID: user.ID})
	sharedAdminMember := builder.ProjectMember(factory.ProjectMemberParams{ProjectID: sharedProject.ID, ProfileID: admin.ID})
	sharedUser1Member := builder.ProjectMember(factory.ProjectMemberParams{ProjectID: sharedProject.ID, ProfileID: user1.ID})

	// user-only project: 3 databases, 1 container, 8 clouds, 2 applications.
	userTypes := []int16{201, 202, 203, 401, 501, 502, 503, 504, 505, 501, 501, 501, 601, 601}
	for i, schemeType := range userTypes {
		alias := fmt.Sprintf("a%03d", i+1)
		s := builder.Scheme(factory.SchemeParams{
			ProjectID:  userProject.ID,
			SchemeType: schemeType,
			Title:      "scheme " + alias,
			Access:     schemeAccess(alias),
		})
		builder.SchemeMember(factory.SchemeMemberParams{SchemeID: s.ID, ProjectMemberID: userMember.ID})
	}

	// shared project: 5 servers, 7 databases, 2 desktops, 2 containers.
	sharedTypes := []int16{103, 103, 103, 103, 103, 205, 205, 205, 205, 205, 205, 205, 301, 301, 401, 401}
	for i, schemeType := range sharedTypes {
		alias := fmt.Sprintf("b%03d", i+1)
		s := builder.Scheme(factory.SchemeParams{
			ProjectID:  sharedProject.ID,
			SchemeType: schemeType,
			Title:      "scheme " + alias,
			Access:     schemeAccess(alias),
		})
		builder.SchemeMember(factory.SchemeMemberParams{SchemeID: s.ID, ProjectMemberID: sharedUserMember.ID})
		builder.SchemeMember(factory.SchemeMemberParams{SchemeID: s.ID, ProjectMemberID: sharedAdminMember.ID})
		if i == 0 {
			builder.SchemeMember(factory.SchemeMemberParams{SchemeID: s.ID, ProjectMemberID: sharedUser1Member.ID})
		}
	}

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := schemepb.NewSchemeHandlersClient(setup)
		return a.ProfileSchemes(ctx, req.(*schemepb.ProfileSchemes_Request))
	}

	testTable := []test.GRPCTable{
		{ // request without parameters
			Name:    "test0_01",
			Request: &schemepb.ProfileSchemes_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"profile_id": "value is empty, which is not a valid UUID",
				},
			},
		},
		{ // user schemes
			Name: "test0_02",
			Request: &schemepb.ProfileSchemes_Request{
				ProfileId: user.ID,
				SortBy:    "alias:ASC",
			},
			Response: test.BodyTable{
				"total.100":            float64(5),
				"total.200":            float64(10),
				"total.300":            float64(2),
				"total.400":            float64(3),
				"total.500":            float64(8),
				"total.600":            float64(2),
				"schemes.0.project_id": userProject.ID,
			},
		},
		{ // user database schemes
			Name: "test0_03",
			Request: &schemepb.ProfileSchemes_Request{
				ProfileId:  user.ID,
				SchemeType: schemeaccesspb.SchemeType_database,
				SortBy:     "alias:ASC",
			},
			Response: test.BodyTable{
				"total.100":            float64(5),
				"total.200":            float64(10),
				"total.300":            float64(2),
				"total.400":            float64(3),
				"total.500":            float64(8),
				"total.600":            float64(2),
				"schemes.0.project_id": userProject.ID,
			},
		},
		{ // admin schemes
			Name: "test0_04",
			Request: &schemepb.ProfileSchemes_Request{
				ProfileId: admin.ID,
			},
			Response: test.BodyTable{
				"total.100": float64(5),
				"total.200": float64(7),
				"total.300": float64(2),
				"total.400": float64(2),
				"total.500": float64(0),
				"total.600": float64(0),
			},
		},
		{ // user1 schemes
			Name: "test0_05",
			Request: &schemepb.ProfileSchemes_Request{
				ProfileId: user1.ID,
			},
			Response: test.BodyTable{
				"total.100": float64(1),
				"total.200": float64(0),
				"total.300": float64(0),
				"total.400": float64(0),
				"total.500": float64(0),
				"total.600": float64(0),
			},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			test.RunCaseGRPCTests(t, handler, tt)
		})
	}
}
