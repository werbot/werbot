package token_test

import (
	"context"
	"testing"
	"time"

	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	tokenpb "github.com/werbot/werbot/internal/core/token/proto/rpc"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/internal/utils/test"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func Test_ProjectTokens(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := tokenpb.NewTokenHandlersClient(setup)
		return a.ProjectTokens(ctx, req.(*tokenmessage.ProjectTokens_Request))
	}

	testTable := []test.GRPCTable{
		{ // request without parameters
			Name:    "test0_01",
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
			Name: "test0_02",
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
			Name: "test0_03",
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
			Name: "test0_04",
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
			Name: "test0_05",
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
			Name: "test0_06",
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
		{ // request without parameters
			Name:    "test0_01",
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
			Name: "test0_02",
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
			Name: "test1_01",
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
			Name: "test1_02",
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
			Name: "test1_03",
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
			Name: "test1_04",
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

		// ------------------------------------------------

		{
			Name: "test2_01",
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
			Name: "test2_02",
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
			Name: "test2_03",
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
			Name: "test2_04",
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
		{ // request without parameters
			Name:    "test0_01",
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
			Name: "test0_02",
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
			Name: "test0_03",
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
			Name: "test0_04",
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
			Name: "test0_05",
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
			Name: "test0_06",
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
			Name: "test0_07",
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
			Name: "test0_08",
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
			Name: "test0_09",
			Request: &tokenmessage.UpdateProjectToken_Request{
				IsAdmin: true,
				Token:   "958301b8-1ed0-4977-9a2a-90629e8a18b9",
				Status:  tokenenum.Status_done,
			},
			Response: test.BodyTable{},
		},
		{
			Name: "test0_10",
			Request: &tokenmessage.UpdateProjectToken_Request{
				IsAdmin:   true,
				Token:     "b5b128d7-ff0c-479c-8216-14eedf9265ad",
				Status:    tokenenum.Status_deleted,
				ProfileId: test.ConstUserID,
			},
			Response: test.BodyTable{},
		},
		{
			Name: "test0_11",
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
