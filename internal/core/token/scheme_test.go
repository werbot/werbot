package token_test

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"

	schemeaccesspb "github.com/werbot/werbot/internal/core/scheme/proto/access"
	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	tokenpb "github.com/werbot/werbot/internal/core/token/proto/rpc"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/internal/utils/test"
)

func Test_SchemeTokens(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := tokenpb.NewTokenHandlersClient(setup)
		return a.SchemeTokens(ctx, req.(*tokenmessage.SchemeTokens_Request))
	}

	testTable := []test.GRPCTable{
		{ // request without parameters
			Name:    "test0_01",
			Request: &tokenmessage.SchemeTokens_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id":  "value is required",
					"scheme_id": "value is required",
					"action":    "value is required",
				},
			},
		},
		{
			Name: "test0_02",
			Request: &tokenmessage.SchemeTokens_Request{
				OwnerId:  "ok",
				SchemeId: "ok",
				Action:   99,
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id":  "value must be a valid UUID",
					"scheme_id": "value must be a valid UUID",
					"action":    "value must be one of the defined enum values",
				},
			},
		},
		{
			Name: "test0_03",
			Request: &tokenmessage.SchemeTokens_Request{
				OwnerId:  test.ConstFakeID,
				SchemeId: test.ConstFakeID,
				Action:   tokenenum.Action_add,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: trace.MsgTokenNotFound,
			},
		},
		{
			Name: "test0_04",
			Request: &tokenmessage.SchemeTokens_Request{
				OwnerId:  test.ConstAdminID,
				SchemeId: test.ConstFakeID,
				Action:   tokenenum.Action_add,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: trace.MsgTokenNotFound,
			},
		},
		{
			Name: "test0_05",
			Request: &tokenmessage.SchemeTokens_Request{
				OwnerId:  test.ConstFakeID,
				SchemeId: "0918e4c3-7f61-4c4e-99ed-800c9af0d265",
				Action:   tokenenum.Action_add,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: trace.MsgTokenNotFound,
			},
		},
		{
			Name: "test0_06",
			Request: &tokenmessage.SchemeTokens_Request{
				OwnerId:  test.ConstAdminID,
				SchemeId: "0918e4c3-7f61-4c4e-99ed-800c9af0d265",
				Action:   tokenenum.Action_add,
			},
			Response: test.BodyTable{
				"total": float64(1),
				// ----
				"tokens.0.action":     float64(3),
				"tokens.0.status":     float64(2),
				"tokens.0.expired_at": "*",
				"tokens.0.updated_at": nil,
				"tokens.0.created_at": "*",
			},
		},
		{
			Name: "test0_07",
			Request: &tokenmessage.SchemeTokens_Request{
				OwnerId:  test.ConstAdminID,
				SchemeId: "0918e4c3-7f61-4c4e-99ed-800c9af0d265",
				Action:   tokenenum.Action_add,
				Status:   tokenenum.Status_done,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: trace.MsgTokenNotFound,
			},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			test.RunCaseGRPCTests(t, handler, tt)
		})
	}
}

func Test_AddTokenSchemeAdd(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := tokenpb.NewTokenHandlersClient(setup)
		return a.AddTokenSchemeAdd(ctx, req.(*tokenmessage.AddTokenSchemeAdd_Request))
	}

	now := time.Now()

	testTable := []test.GRPCTable{
		{ // request without parameters
			Name:    "test0_01",
			Request: &tokenmessage.AddTokenSchemeAdd_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id":   "value is required",
					"project_id": "value is required",
					"data":       "exactly one field is required in oneof",
				},
			},
		},

		{
			Name: "test0_02",
			Request: &tokenmessage.AddTokenSchemeAdd_Request{
				SchemeType: schemeaccesspb.SchemeType_server_ssh,
				Data: &tokenmessage.AddTokenSchemeAdd_Request_Email{
					Email: "admin@mail.com",
				},
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id":   "value is required",
					"project_id": "value is required",
				},
			},
		},
		{
			Name: "test0_03",
			Request: &tokenmessage.AddTokenSchemeAdd_Request{
				ProjectId:  "ok",
				SchemeType: schemeaccesspb.SchemeType_server_ssh,
				Data: &tokenmessage.AddTokenSchemeAdd_Request_Email{
					Email: "admin@mail.com",
				},
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"project_id": "value must be a valid UUID",
				},
			},
		},
		{
			Name: "test0_04",
			Request: &tokenmessage.AddTokenSchemeAdd_Request{
				OwnerId:   test.ConstAdminID,
				ProjectId: test.ConstFakeID,
				Data: &tokenmessage.AddTokenSchemeAdd_Request_Email{
					Email: "admin",
				},
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"email": "must be a valid email",
				},
			},
		},
		{
			Name: "test0_05",
			Request: &tokenmessage.AddTokenSchemeAdd_Request{
				OwnerId:   test.ConstAdminID,
				ProjectId: test.ConstFakeID,
				Data: &tokenmessage.AddTokenSchemeAdd_Request_Email{
					Email: "admin@mail.com",
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Not found",
			},
		},
		{
			Name: "test0_06",
			Request: &tokenmessage.AddTokenSchemeAdd_Request{
				OwnerId:   test.ConstFakeID,
				ProjectId: "fe52ca9b-5599-4bb6-818b-1896d56e9aa2",
				Data: &tokenmessage.AddTokenSchemeAdd_Request_Email{
					Email: "admin@mail.com",
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Not found",
			},
		},
		{
			Name: "test0_07",
			Request: &tokenmessage.AddTokenSchemeAdd_Request{
				OwnerId:    test.ConstAdminID,
				ProjectId:  "fe52ca9b-5599-4bb6-818b-1896d56e9aa2",
				SchemeType: schemeaccesspb.SchemeType_server_ssh,
				Data: &tokenmessage.AddTokenSchemeAdd_Request_Email{
					Email: "admin@mail.com",
				},
			},
			Response: test.BodyTable{
				"token": "*",
			},
		},
		{
			Name: "test0_08",
			Request: &tokenmessage.AddTokenSchemeAdd_Request{
				OwnerId:    test.ConstAdminID,
				ProjectId:  "fe52ca9b-5599-4bb6-818b-1896d56e9aa2",
				SchemeType: schemeaccesspb.SchemeType_server_ssh,
				Data: &tokenmessage.AddTokenSchemeAdd_Request_Email{
					Email: "admin@mail.com", // profile not registered
				},
			},
			Response: test.BodyTable{
				"token": "*",
			},
		},
		{
			Name: "test0_09",
			Request: &tokenmessage.AddTokenSchemeAdd_Request{
				OwnerId:    test.ConstAdminID,
				ProjectId:  "fe52ca9b-5599-4bb6-818b-1896d56e9aa2",
				SchemeType: schemeaccesspb.SchemeType_server_ssh,
				Data: &tokenmessage.AddTokenSchemeAdd_Request_Email{
					Email: "admin@werbot.net", // the profile is registered and has a profile UUID
				},
				ExpiredAt: timestamppb.New(now.Add(36 * time.Hour)),
			},
			Response: test.BodyTable{
				"token": "*",
			},
		},

		{ // broken profile UUID
			Name: "test0_10",
			Request: &tokenmessage.AddTokenSchemeAdd_Request{
				OwnerId:    test.ConstAdminID,
				ProjectId:  "fe52ca9b-5599-4bb6-818b-1896d56e9aa2",
				SchemeType: schemeaccesspb.SchemeType_server_ssh,
				Data: &tokenmessage.AddTokenSchemeAdd_Request_ProfileId{
					ProfileId: test.ConstFakeID,
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Not found",
			},
		},
		{ // broken profile UUID
			Name: "test0_11",
			Request: &tokenmessage.AddTokenSchemeAdd_Request{
				OwnerId:    test.ConstAdminID,
				ProjectId:  "fe52ca9b-5599-4bb6-818b-1896d56e9aa2",
				SchemeType: schemeaccesspb.SchemeType_server_ssh,
				Data: &tokenmessage.AddTokenSchemeAdd_Request_ProfileId{
					ProfileId: "ok",
				},
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"profile_id": "value must be a valid UUID",
				},
			},
		},
		{ // broken profile UUID
			Name: "test0_12",
			Request: &tokenmessage.AddTokenSchemeAdd_Request{
				OwnerId:    "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				ProjectId:  "fe52ca9b-5599-4bb6-818b-1896d56e9aa2",
				SchemeType: schemeaccesspb.SchemeType_server_ssh,
				Data: &tokenmessage.AddTokenSchemeAdd_Request_ProfileId{
					ProfileId: "51c12bb6-2da6-491d-8003-b024f54a1491",
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

func Test_AddTokenSchemeAccess(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := tokenpb.NewTokenHandlersClient(setup)
		return a.AddTokenSchemeAccess(ctx, req.(*tokenmessage.AddTokenSchemeAccess_Request))
	}

	now := time.Now()

	testTable := []test.GRPCTable{
		{ // request without parameters
			Name:    "test0_01",
			Request: &tokenmessage.AddTokenSchemeAccess_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id":  "value is required",
					"scheme_id": "value is required",
					"data":      "exactly one field is required in oneof",
				},
			},
		},

		{
			Name: "test0_02",
			Request: &tokenmessage.AddTokenSchemeAccess_Request{
				Data: &tokenmessage.AddTokenSchemeAccess_Request_Email{
					Email: "admin@mail.com",
				},
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id":  "value is required",
					"scheme_id": "value is required",
				},
			},
		},
		{
			Name: "test0_03",
			Request: &tokenmessage.AddTokenSchemeAccess_Request{
				OwnerId:  "ok",
				SchemeId: "ok",
				Data: &tokenmessage.AddTokenSchemeAccess_Request_Email{
					Email: "admin",
				},
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id":  "value must be a valid UUID",
					"scheme_id": "value must be a valid UUID",
					"email":     "must be a valid email",
				},
			},
		},
		{
			Name: "test0_04",
			Request: &tokenmessage.AddTokenSchemeAccess_Request{
				OwnerId:  test.ConstAdminID,
				SchemeId: test.ConstFakeID,
				Data: &tokenmessage.AddTokenSchemeAccess_Request_Email{
					Email: "admin",
				},
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"email": "must be a valid email",
				},
			},
		},
		{
			Name: "test0_05",
			Request: &tokenmessage.AddTokenSchemeAccess_Request{
				OwnerId:  test.ConstAdminID,
				SchemeId: test.ConstFakeID,
				Data: &tokenmessage.AddTokenSchemeAccess_Request_Email{
					Email: "admin@mail.com",
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Not found",
			},
		},

		{
			Name: "test0_06",
			Request: &tokenmessage.AddTokenSchemeAccess_Request{
				OwnerId:  test.ConstFakeID,
				SchemeId: test.ConstAdminSchemeSSH1ID,
				Data: &tokenmessage.AddTokenSchemeAccess_Request_Email{
					Email: "user@mail.com",
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Not found",
			},
		},
		{
			Name: "test0_07",
			Request: &tokenmessage.AddTokenSchemeAccess_Request{
				OwnerId:  test.ConstAdminID,
				SchemeId: test.ConstAdminSchemeSSH1ID,
				Data: &tokenmessage.AddTokenSchemeAccess_Request_Email{
					Email: "user@mail.com",
				},
			},
			Response: test.BodyTable{
				"token": "*",
			},
		},
		{
			Name: "test0_08",
			Request: &tokenmessage.AddTokenSchemeAccess_Request{
				OwnerId:  test.ConstAdminID,
				SchemeId: test.ConstAdminSchemeSSH1ID,
				Data: &tokenmessage.AddTokenSchemeAccess_Request_ProfileId{
					ProfileId: test.ConstFakeID,
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Not found",
			},
		},
		{
			Name: "test0_09",
			Request: &tokenmessage.AddTokenSchemeAccess_Request{
				OwnerId:  test.ConstAdminID,
				SchemeId: test.ConstAdminSchemeSSH1ID,
				Data: &tokenmessage.AddTokenSchemeAccess_Request_ProfileId{
					ProfileId: "c180ad5c-0c65-4cee-8725-12931cb5abb3",
				},
			},
			Response: test.BodyTable{
				"token": "*",
			},
		},
		{
			Name: "test0_10",
			Request: &tokenmessage.AddTokenSchemeAccess_Request{
				OwnerId:  test.ConstAdminID,
				SchemeId: test.ConstAdminSchemeSSH1ID,
				Data: &tokenmessage.AddTokenSchemeAccess_Request_ProfileId{
					ProfileId: "c180ad5c-0c65-4cee-8725-12931cb5abb3",
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

func Test_UpdateSchemeToken(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := tokenpb.NewTokenHandlersClient(setup)
		return a.UpdateSchemeToken(ctx, req.(*tokenmessage.UpdateSchemeToken_Request))
	}

	testTable := []test.GRPCTable{
		{ // request without parameters
			Name:    "test0_01",
			Request: &tokenmessage.UpdateSchemeToken_Request{},
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
			Request: &tokenmessage.UpdateSchemeToken_Request{
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
			Request: &tokenmessage.UpdateSchemeToken_Request{
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
			Request: &tokenmessage.UpdateSchemeToken_Request{
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
			Request: &tokenmessage.UpdateSchemeToken_Request{
				Token:  test.ConstFakeID,
				Status: tokenenum.Status_done,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: trace.MsgTokenNotFound,
			},
		},

		/*
			// SCHEME setion
			{ // 3-3-1, ERROR
				Name: "test3_01",
				Request: &tokenmessage.UpdateToken_Request{
					Token:    test.ConstFakeID,
					Status:   tokenenum.Status_done,
					SchemeId: test.ConstFakeID,
				},
				Error: test.ErrGRPC{
					Code:    codes.NotFound,
					Message: trace.MsgTokenNotFound,
				},
			},
			{ // 3-3-1, ERROR
				Name: "test3_02",
				Request: &tokenmessage.UpdateToken_Request{
					Token:  "0a7b333e-4c98-4edd-9e34-a4e734a5926e",
					Status: tokenenum.Status_done,
				},
				Error: test.ErrGRPC{
					Code: codes.InvalidArgument,
					Message: map[string]any{
						"owner_id": "value is required",
					},
				},
			},
			{ // 3-2-1, ERROR
				Name: "test3_03",
				Request: &tokenmessage.UpdateToken_Request{
					OwnerId: test.ConstFakeID,
					Token:   "0a7b333e-4c98-4edd-9e34-a4e734a5926e",
					Status:  tokenenum.Status_done,
				},
				Error: test.ErrGRPC{
					Code:    codes.NotFound,
					Message: "Owner not found",
				},
			},
			{ // 3-2-1, DONE
				Name: "test3_04",
				Request: &tokenmessage.UpdateToken_Request{
					OwnerId: test.ConstAdminID,
					Token:   "0a7b333e-4c98-4edd-9e34-a4e734a5926e",
					Status:  tokenenum.Status_done,
				},
				Response: test.BodyTable{},
			},
			{ // 3-2-1, DONE
				Name: "test3_05",
				Request: &tokenmessage.UpdateToken_Request{
					OwnerId:   test.ConstAdminID,
					Token:     "0a7b333e-4c98-4edd-9e34-a4e734a5926e",
					Status:    tokenenum.Status_done,
					ProfileId: test.ConstUserID,
				},
				Response: test.BodyTable{},
			},

			{ // 3-8-1, DONE
				Name: "test3_07",
				Request: &tokenmessage.UpdateToken_Request{
					OwnerId: test.ConstAdminID,
					Token:   "cde3dd1e-1643-4561-8e5b-e4684c05f595",
					Status:  tokenenum.Status_done,
				},
				Response: test.BodyTable{},
			},
		*/
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			test.RunCaseGRPCTests(t, handler, tt)
		})
	}
}
