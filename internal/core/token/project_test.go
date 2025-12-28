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
)

func Test_ProjectTokens(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

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
				ProjectId: "fe52ca9b-5599-4bb6-818b-1896d56e9aa2",
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
				OwnerId:   test.ConstAdminID,
				ProjectId: "fe52ca9b-5599-4bb6-818b-1896d56e9aa2",
				Action:    tokenenum.Action_request,
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
				OwnerId:   test.ConstAdminID,
				ProjectId: "fe52ca9b-5599-4bb6-818b-1896d56e9aa2",
				Action:    tokenenum.Action_request,
				Status:    tokenenum.Status_sent,
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
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

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
				Message: "Not found",
			},
		},
		{
			Name: "profile_not_found_wrong_owner",
			Request: &tokenmessage.AddTokenProjectMember_Request{
				OwnerId:   test.ConstFakeID,
				ProjectId: "fe52ca9b-5599-4bb6-818b-1896d56e9aa2",
				Profile: &tokenmessage.AddTokenProjectMember_Request_ProfileId{
					ProfileId: test.ConstFakeID,
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Not found",
			},
		},
		{
			Name: "profile_not_found_admin_wrong_profile",
			Request: &tokenmessage.AddTokenProjectMember_Request{
				OwnerId:   test.ConstAdminID,
				ProjectId: "fe52ca9b-5599-4bb6-818b-1896d56e9aa2",
				Profile: &tokenmessage.AddTokenProjectMember_Request_ProfileId{
					ProfileId: test.ConstFakeID,
				},
				ExpiredAt: timestamppb.New(now.Add(36 * time.Hour)),
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Not found",
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
				Message: "Not found",
			},
		},
		{
			Name: "project_not_found_wrong_owner",
			Request: &tokenmessage.AddTokenProjectMember_Request{
				OwnerId:   test.ConstFakeID,
				ProjectId: "fe52ca9b-5599-4bb6-818b-1896d56e9aa2",
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
				Message: "Not found",
			},
		},
		{
			Name: "create_member_token_with_new_profile_success",
			Request: &tokenmessage.AddTokenProjectMember_Request{
				OwnerId:   test.ConstAdminID,
				ProjectId: "fe52ca9b-5599-4bb6-818b-1896d56e9aa2",
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
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

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
				Token:  "b5b128d7-ff0c-479c-8216-14eedf9265ad",
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
				Token:     "b5b128d7-ff0c-479c-8216-14eedf9265ad",
				Status:    tokenenum.Status_deleted,
				ProfileId: test.ConstUserID,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: trace.MsgStatusNotFound,
			},
		},
		{
			Name: "user_cannot_update_other_owner_token",
			Request: &tokenmessage.UpdateProjectToken_Request{
				Token:  "958301b8-1ed0-4977-9a2a-90629e8a18b9",
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
				Token:   "958301b8-1ed0-4977-9a2a-90629e8a18b9",
				Status:  tokenenum.Status_done,
			},
			Response: test.BodyTable{},
		},
		{
			Name: "admin_set_deleted_status_success",
			Request: &tokenmessage.UpdateProjectToken_Request{
				IsAdmin:   true,
				Token:     "b5b128d7-ff0c-479c-8216-14eedf9265ad",
				Status:    tokenenum.Status_deleted,
				ProfileId: test.ConstUserID,
			},
			Response: test.BodyTable{},
		},
		{
			Name: "user_update_member_token_success",
			Request: &tokenmessage.UpdateProjectToken_Request{
				Token:     "b5b128d7-ff0c-479c-8216-14eedf9265ad",
				Status:    tokenenum.Status_done,
				ProfileId: test.ConstUserID,
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
