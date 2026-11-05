package token_test

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"

	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	tokenpb "github.com/werbot/werbot/internal/core/token/proto/rpc"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/internal/utils/test/factory"
)

func Test_ProjectTokens(t *testing.T) {
	setup, teardownTestCase := test.GRPCWithoutFixtures(t)
	defer teardownTestCase(t)

	builder := factory.New(t, setup.Postgres.Conn().Conn)

	admin := builder.Profile(factory.ProfileParams{Alias: "admin", Role: 3})
	project := builder.Project(factory.ProjectParams{OwnerID: admin.ID, Title: "project1"})

	now := time.Now().UTC()
	builder.Token(factory.TokenParams{
		Section:   2,
		Action:    7,
		Status:    2,
		ProjectID: project.ID,
		ExpiredAt: factory.Time(now.Add(24 * time.Hour)),
		CreatedAt: factory.Time(now.Add(-2 * time.Second)),
	})
	builder.Token(factory.TokenParams{
		Section:   2,
		Action:    7,
		Status:    1,
		ProjectID: project.ID,
		CreatedAt: factory.Time(now.Add(-time.Second)),
	})

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := tokenpb.NewTokenHandlersClient(setup)
		return a.ProjectTokens(ctx, req.(*tokenmessage.ProjectTokens_Request))
	}

	testTable := []test.GRPCTable{
		{
			Name:    "missing_required_parameters",
			Request: &tokenmessage.ProjectTokens_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id":   "value is required",
					"project_id": "value is required",
					"action":     "value is required",
				},
			},
		},
		{
			Name: "invalid_parameters_format",
			Request: &tokenmessage.ProjectTokens_Request{
				OwnerId:   "ok",
				ProjectId: "ok",
				Action:    99,
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id":   "value must be a valid UUID",
					"project_id": "value must be a valid UUID",
					"action":     "value must be one of the defined enum values",
				},
			},
		},
		{
			Name: "tokens_not_found_wrong_owner_and_project",
			Request: &tokenmessage.ProjectTokens_Request{
				OwnerId:   test.ConstFakeID,
				ProjectId: test.ConstFakeID,
				Action:    tokenenum.Action_request,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: trace.MsgTokenNotFound,
			},
		},
		{
			Name: "tokens_not_found_wrong_owner",
			Request: &tokenmessage.ProjectTokens_Request{
				OwnerId:   test.ConstFakeID,
				ProjectId: project.ID,
				Action:    tokenenum.Action_request,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: trace.MsgTokenNotFound,
			},
		},
		{
			Name: "admin_list_project_tokens_success",
			Request: &tokenmessage.ProjectTokens_Request{
				OwnerId:   admin.ID,
				ProjectId: project.ID,
				Action:    tokenenum.Action_request,
				SortBy:    "created_at:ASC",
			},
			Response: test.BodyTable{
				"total": float64(2),
				// ----
				"tokens.0.action":     float64(7),
				"tokens.0.status":     float64(2),
				"tokens.0.expired_at": "*",
				"tokens.0.updated_at": nil,
				"tokens.0.created_at": "*",
				// ----
				"tokens.1.action":     float64(7),
				"tokens.1.status":     float64(1),
				"tokens.1.expired_at": nil,
				"tokens.1.updated_at": nil,
				"tokens.1.created_at": "*",
				// ----
				"tokens.2.action": nil,
			},
		},
		{
			Name: "admin_list_project_tokens_with_status_filter",
			Request: &tokenmessage.ProjectTokens_Request{
				OwnerId:   admin.ID,
				ProjectId: project.ID,
				Action:    tokenenum.Action_request,
				Status:    tokenenum.Status_sent,
				SortBy:    "created_at:ASC",
			},
			Response: test.BodyTable{
				"total": float64(1),
				// ----
				"tokens.0.action":     float64(7),
				"tokens.0.status":     float64(2),
				"tokens.0.expired_at": "*",
				"tokens.0.updated_at": nil,
				"tokens.0.created_at": "*",
			},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			test.RunCaseGRPCTests(t, handler, tt)
		})
	}
}

func Test_AddTokenProjectMember(t *testing.T) {
	setup, teardownTestCase := test.GRPCWithoutFixtures(t)
	defer teardownTestCase(t)

	builder := factory.New(t, setup.Postgres.Conn().Conn)

	admin := builder.Profile(factory.ProfileParams{Alias: "admin", Role: 3})
	project := builder.Project(factory.ProjectParams{OwnerID: admin.ID, Title: "project1"})

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := tokenpb.NewTokenHandlersClient(setup)
		return a.AddTokenProjectMember(ctx, req.(*tokenmessage.AddTokenProjectMember_Request))
	}

	now := time.Now()

	testTable := []test.GRPCTable{
		{
			Name:    "missing_required_parameters",
			Request: &tokenmessage.AddTokenProjectMember_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id":   "value is required",
					"project_id": "value is required",
					"profile":    "exactly one field is required in oneof",
				},
			},
		},
		{
			Name: "invalid_parameters_format_with_profile_id",
			Request: &tokenmessage.AddTokenProjectMember_Request{
				OwnerId:   "ok",
				ProjectId: "ok",
				Profile: &tokenmessage.AddTokenProjectMember_Request_ProfileId{
					ProfileId: "ok",
				},
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id":   "value must be a valid UUID",
					"project_id": "value must be a valid UUID",
					"profile_id": "value must be a valid UUID",
				},
			},
		},
		{
			Name: "profile_not_found_wrong_owner_and_project",
			Request: &tokenmessage.AddTokenProjectMember_Request{
				OwnerId:   test.ConstFakeID,
				ProjectId: test.ConstFakeID,
				Profile: &tokenmessage.AddTokenProjectMember_Request_ProfileId{
					ProfileId: test.ConstFakeID,
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Project not found",
			},
		},
		{
			Name: "profile_not_found_wrong_owner",
			Request: &tokenmessage.AddTokenProjectMember_Request{
				OwnerId:   test.ConstFakeID,
				ProjectId: project.ID,
				Profile: &tokenmessage.AddTokenProjectMember_Request_ProfileId{
					ProfileId: test.ConstFakeID,
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Project not found",
			},
		},
		{
			Name: "profile_not_found_admin_wrong_profile",
			Request: &tokenmessage.AddTokenProjectMember_Request{
				OwnerId:   admin.ID,
				ProjectId: project.ID,
				Profile: &tokenmessage.AddTokenProjectMember_Request_ProfileId{
					ProfileId: test.ConstFakeID,
				},
				ExpiredAt: timestamppb.New(now.Add(36 * time.Hour)),
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Profile not found",
			},
		},
		{
			Name: "invalid_parameters_format_with_new_profile",
			Request: &tokenmessage.AddTokenProjectMember_Request{
				OwnerId:   "ok",
				ProjectId: "ok",
				Profile: &tokenmessage.AddTokenProjectMember_Request_CreateNewProfile{
					CreateNewProfile: &tokenmessage.MetaDataProfile{
						Name:    "ok",
						Surname: "ok",
						Email:   "ok",
					},
				},
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id":                   "value must be a valid UUID",
					"project_id":                 "value must be a valid UUID",
					"create_new_profile.name":    "required field (3 to 30 characters)",
					"create_new_profile.surname": "required field (3 to 30 characters)",
					"create_new_profile.email":   "must be a valid email",
				},
			},
		},
		{
			Name: "project_not_found_wrong_owner_and_project",
			Request: &tokenmessage.AddTokenProjectMember_Request{
				OwnerId:   test.ConstFakeID,
				ProjectId: test.ConstFakeID,
				Profile: &tokenmessage.AddTokenProjectMember_Request_CreateNewProfile{
					CreateNewProfile: &tokenmessage.MetaDataProfile{
						Name:    "name",
						Surname: "surname",
						Email:   "user00@mail.com",
					},
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Project not found",
			},
		},
		{
			Name: "project_not_found_wrong_owner",
			Request: &tokenmessage.AddTokenProjectMember_Request{
				OwnerId:   test.ConstFakeID,
				ProjectId: project.ID,
				Profile: &tokenmessage.AddTokenProjectMember_Request_CreateNewProfile{
					CreateNewProfile: &tokenmessage.MetaDataProfile{
						Name:    "name",
						Surname: "surname",
						Email:   "user00@mail.com",
					},
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Project not found",
			},
		},
		{
			Name: "create_member_token_with_new_profile_success",
			Request: &tokenmessage.AddTokenProjectMember_Request{
				OwnerId:   admin.ID,
				ProjectId: project.ID,
				Profile: &tokenmessage.AddTokenProjectMember_Request_CreateNewProfile{
					CreateNewProfile: &tokenmessage.MetaDataProfile{
						Name:    "name",
						Surname: "surname",
						Email:   "user00@mail.com",
					},
				},
				ExpiredAt: timestamppb.New(now.Add(36 * time.Hour)),
			},
			Response: test.BodyTable{
				"token": "*",
			},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			test.RunCaseGRPCTests(t, handler, tt)
		})
	}
}

func Test_UpdateProjectToken(t *testing.T) {
	setup, teardownTestCase := test.GRPCWithoutFixtures(t)
	defer teardownTestCase(t)

	builder := factory.New(t, setup.Postgres.Conn().Conn)

	admin := builder.Profile(factory.ProfileParams{Alias: "admin", Role: 3})
	user := builder.Profile(factory.ProfileParams{Alias: "user", Name: "user", Surname: "user", Email: "user@werbot.net"})
	project := builder.Project(factory.ProjectParams{OwnerID: admin.ID, Title: "project1"})

	memberToken := builder.Token(factory.TokenParams{
		Section:   2,
		Action:    7,
		Status:    2,
		ProjectID: project.ID,
	})
	doneToken := builder.Token(factory.TokenParams{
		Section:   2,
		Action:    7,
		Status:    1,
		ProfileID: user.ID,
		ProjectID: project.ID,
		Data:      map[string]any{"name": "test3", "surname": "test3", "email": "test3@werbot.net"},
	})

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := tokenpb.NewTokenHandlersClient(setup)
		return a.UpdateProjectToken(ctx, req.(*tokenmessage.UpdateProjectToken_Request))
	}

	testTable := []test.GRPCTable{
		{
			Name:    "missing_token_and_status",
			Request: &tokenmessage.UpdateProjectToken_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"token":  "value is required",
					"status": "value is required",
				},
			},
		},
		{
			Name: "invalid_token_format_and_status_enum",
			Request: &tokenmessage.UpdateProjectToken_Request{
				Token:  "ok",
				Status: 99,
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"token":  "value must be a valid UUID",
					"status": "value must be one of the defined enum values",
				},
			},
		},
		{
			Name: "missing_status",
			Request: &tokenmessage.UpdateProjectToken_Request{
				Token: test.ConstFakeID,
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"status": "value is required",
				},
			},
		},
		{
			Name: "token_not_found",
			Request: &tokenmessage.UpdateProjectToken_Request{
				Token:  test.ConstFakeID,
				Status: tokenenum.Status_done,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: trace.MsgTokenNotFound,
			},
		},
		{
			Name: "token_not_found_duplicate",
			Request: &tokenmessage.UpdateProjectToken_Request{
				Token:  test.ConstFakeID,
				Status: tokenenum.Status_done,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: trace.MsgTokenNotFound,
			},
		},
		{
			Name: "member_token_missing_profile_id",
			Request: &tokenmessage.UpdateProjectToken_Request{
				Token:  memberToken.ID,
				Status: tokenenum.Status_done,
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"profile_id": "value is required",
				},
			},
		},
		{
			Name: "user_cannot_set_deleted_status",
			Request: &tokenmessage.UpdateProjectToken_Request{
				Token:     memberToken.ID,
				Status:    tokenenum.Status_deleted,
				ProfileId: user.ID,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: trace.MsgStatusNotFound,
			},
		},
		{
			Name: "user_cannot_update_other_owner_token",
			Request: &tokenmessage.UpdateProjectToken_Request{
				Token:  doneToken.ID,
				Status: tokenenum.Status_done,
			},
			Error: test.ErrGRPC{
				Code:    codes.PermissionDenied,
				Message: trace.MsgPermissionDenied,
			},
		},
		{
			Name: "admin_update_token_success",
			Request: &tokenmessage.UpdateProjectToken_Request{
				IsAdmin: true,
				Token:   doneToken.ID,
				Status:  tokenenum.Status_done,
			},
			Response: test.BodyTable{},
		},
		{
			Name: "admin_set_deleted_status_success",
			Request: &tokenmessage.UpdateProjectToken_Request{
				IsAdmin:   true,
				Token:     memberToken.ID,
				Status:    tokenenum.Status_deleted,
				ProfileId: user.ID,
			},
			Response: test.BodyTable{},
		},
		{
			Name: "user_update_member_token_success",
			Request: &tokenmessage.UpdateProjectToken_Request{
				Token:     memberToken.ID,
				Status:    tokenenum.Status_done,
				ProfileId: user.ID,
			},
			Response: test.BodyTable{},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			test.RunCaseGRPCTests(t, handler, tt)
		})
	}
}
