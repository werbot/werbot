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

func Test_ProfileTokens(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := tokenpb.NewTokenHandlersClient(setup)
		return a.ProfileTokens(ctx, req.(*tokenmessage.ProfileTokens_Request))
	}

	testTable := []test.GRPCTable{
		{
			Name:    "test0_01",
			Request: &tokenmessage.ProfileTokens_Request{},
			Error: test.ErrGRPC{
				Code:    codes.PermissionDenied,
				Message: "Permission denied",
			},
		},
		{
			Name: "test0_02",
			Request: &tokenmessage.ProfileTokens_Request{
				IsAdmin: true,
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"action": "value is required",
				},
			},
		},
		{
			Name: "test0_03",
			Request: &tokenmessage.ProfileTokens_Request{
				IsAdmin: true,
				Action:  99,
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"action": "value must be one of the defined enum values",
				},
			},
		},
		{
			Name: "test0_04",
			Request: &tokenmessage.ProfileTokens_Request{
				IsAdmin: true,
				Action:  tokenenum.Action_register,
			},
			Response: test.BodyTable{
				"total": float64(9),
				// ----
				"tokens.0.status":     float64(1),
				"tokens.0.expired_at": "*",
				"tokens.0.updated_at": "*",
				"tokens.0.created_at": "*",
				// ----
				"tokens.1.status":     float64(2),
				"tokens.1.expired_at": "*",
				"tokens.1.updated_at": nil,
				"tokens.1.created_at": "*",
				// ----
				"tokens.9.action": nil,
			},
		},
		{
			Name: "test0_05",
			Request: &tokenmessage.ProfileTokens_Request{
				IsAdmin: true,
				Action:  tokenenum.Action_register,
				Status:  tokenenum.Status_done,
			},
			Response: test.BodyTable{
				"total": float64(2),
				// ----
				"tokens.0.action":     float64(2),
				"tokens.0.status":     float64(1),
				"tokens.0.expired_at": "*",
				"tokens.0.updated_at": "*",
				"tokens.0.created_at": "*",
				// ----
				"tokens.1.action":     float64(2),
				"tokens.1.status":     float64(1),
				"tokens.1.expired_at": "*",
				"tokens.1.updated_at": "*",
				"tokens.1.created_at": "*",
				// ----
				"tokens.2.action": nil,
			},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			test.RunCaseGRPCTests(t, handler, tt)
		})
	}
}

func Test_AddTokenProfileReset(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := tokenpb.NewTokenHandlersClient(setup)
		return a.AddTokenProfileReset(ctx, req.(*tokenmessage.AddTokenProfileReset_Request))
	}

	now := time.Now()

	testTable := []test.GRPCTable{
		{ // request without parameters
			Name:    "test0_01",
			Request: &tokenmessage.AddTokenProfileReset_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"profile_id": "value is required",
				},
			},
		},
		{
			Name: "test0_02",
			Request: &tokenmessage.AddTokenProfileReset_Request{
				ProfileId: "ok",
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"profile_id": "value must be a valid UUID",
				},
			},
		},
		{
			Name: "test0_03",
			Request: &tokenmessage.AddTokenProfileReset_Request{
				ProfileId: test.ConstFakeID,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Not found",
			},
		},
		{
			Name: "test0_04",
			Request: &tokenmessage.AddTokenProfileReset_Request{
				ProfileId: test.ConstUserID,
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

func Test_AddTokenProfileRegistration(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := tokenpb.NewTokenHandlersClient(setup)
		return a.AddTokenProfileRegistration(ctx, req.(*tokenmessage.AddTokenProfileRegistration_Request))
	}

	now := time.Now()

	testTable := []test.GRPCTable{
		{ // request without parameters
			Name:    "test0_01",
			Request: &tokenmessage.AddTokenProfileRegistration_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"data": "value is required",
				},
			},
		},

		{
			Name: "test0_02",
			Request: &tokenmessage.AddTokenProfileRegistration_Request{
				Data: &tokenmessage.MetaDataProfile{},
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"data.name":    "value is required",
					"data.surname": "value is required",
					"data.email":   "value is required",
				},
			},
		},
		{
			Name: "test0_03",
			Request: &tokenmessage.AddTokenProfileRegistration_Request{
				Data: &tokenmessage.MetaDataProfile{
					Name:    "ok",
					Surname: "ok",
					Email:   "ok",
				},
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"data.name":    "required field (3 to 30 characters)",
					"data.surname": "required field (3 to 30 characters)",
					"data.email":   "must be a valid email",
				},
			},
		},
		{
			Name: "test0_04",
			Request: &tokenmessage.AddTokenProfileRegistration_Request{
				Data: &tokenmessage.MetaDataProfile{
					Name:    "name",
					Surname: "surname",
					Email:   "user00@mail.com",
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

func Test_AddTokenProfileDelete(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := tokenpb.NewTokenHandlersClient(setup)
		return a.AddTokenProfileDelete(ctx, req.(*tokenmessage.AddTokenProfileDelete_Request))
	}

	now := time.Now()

	testTable := []test.GRPCTable{
		{ // request without parameters
			Name:    "test0_01",
			Request: &tokenmessage.AddTokenProfileDelete_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"profile_id": "value is required",
				},
			},
		},

		{
			Name: "test0_02",
			Request: &tokenmessage.AddTokenProfileDelete_Request{
				ProfileId: "ok",
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"profile_id": "value must be a valid UUID",
				},
			},
		},
		{
			Name: "test0_03",
			Request: &tokenmessage.AddTokenProfileDelete_Request{
				ProfileId: test.ConstFakeID,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Not found",
			},
		},
		{
			Name: "test0_04",
			Request: &tokenmessage.AddTokenProfileDelete_Request{
				ProfileId: test.ConstUserID,
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

func Test_UpdateProfileToken(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := tokenpb.NewTokenHandlersClient(setup)
		return a.UpdateProfileToken(ctx, req.(*tokenmessage.UpdateProfileToken_Request))
	}

	testTable := []test.GRPCTable{
		{ // request without parameters
			Name:    "test0_01",
			Request: &tokenmessage.UpdateProfileToken_Request{},
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
			Request: &tokenmessage.UpdateProfileToken_Request{
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
			Request: &tokenmessage.UpdateProfileToken_Request{
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
			Request: &tokenmessage.UpdateProfileToken_Request{
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
			Request: &tokenmessage.UpdateProfileToken_Request{
				Token:     test.ConstFakeID,
				Status:    tokenenum.Status_done,
				ProfileId: test.ConstFakeID,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: trace.MsgTokenNotFound,
			},
		},
		{
			Name: "test0_06",
			Request: &tokenmessage.UpdateProfileToken_Request{
				Token:  "55c9f79c-d827-43fc-8ad1-79e396d2432c",
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
			Request: &tokenmessage.UpdateProfileToken_Request{
				ProfileId: test.ConstFakeID,
				Token:     "55c9f79c-d827-43fc-8ad1-79e396d2432c",
				Status:    tokenenum.Status_done,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Profile not found",
			},
		},
		{
			Name: "test0_08",
			Request: &tokenmessage.UpdateProfileToken_Request{
				ProfileId: "5a2bb353-e862-4e6f-be74-fedb2dd761fd",
				Token:     "55c9f79c-d827-43fc-8ad1-79e396d2432c",
				Status:    tokenenum.Status_done,
			},
			Response: test.BodyTable{},
		},

		{
			Name: "test0_09",
			Request: &tokenmessage.UpdateProfileToken_Request{
				Token:  "55c9f79c-d827-43fc-8ad1-79e396d2432c",
				Status: tokenenum.Status_used,
			},
			Response: test.BodyTable{},
		},
		{
			Name: "test0_10",
			Request: &tokenmessage.UpdateProfileToken_Request{
				Token:  "55c9f79c-d827-43fc-8ad1-79e396d2432c",
				Status: tokenenum.Status_deleted,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Status not found",
			},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			test.RunCaseGRPCTests(t, handler, tt)
		})
	}
}
