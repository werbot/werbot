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

func Test_ProfileTokens(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	builder := factory.New(t, setup.Postgres.Conn().Conn)

	now := time.Now().UTC()
	expiredAt := factory.Time(now.Add(24 * time.Hour))

	builder.Token(factory.TokenParams{
		Section:   1,
		Action:    2,
		Status:    1,
		Data:      map[string]any{"name": "Terry", "surname": "Henry", "email": "user2@werbot.net"},
		ExpiredAt: expiredAt,
		UpdatedAt: factory.Time(now),
		CreatedAt: factory.Time(now.Add(-2 * time.Second)),
	})
	builder.Token(factory.TokenParams{
		Section:   1,
		Action:    2,
		Status:    2,
		Data:      map[string]any{"name": "invite201", "surname": "invite201", "email": "invite201@werbot.net"},
		ExpiredAt: expiredAt,
		CreatedAt: factory.Time(now.Add(-time.Second)),
	})

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := tokenpb.NewTokenHandlersClient(setup)
		return a.ProfileTokens(ctx, req.(*tokenmessage.ProfileTokens_Request))
	}

	testTable := []test.GRPCTable{
		{
			Name:    "non_admin_access_denied",
			Request: &tokenmessage.ProfileTokens_Request{},
			Error: test.ErrGRPC{
				Code:    codes.PermissionDenied,
				Message: "Permission denied",
			},
		},
		{
			Name: "admin_missing_action",
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
			Name: "admin_invalid_action_enum",
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
			Name: "admin_list_register_tokens",
			Request: &tokenmessage.ProfileTokens_Request{
				IsAdmin: true,
				Action:  tokenenum.Action_register,
				SortBy:  "created_at:ASC",
			},
			Response: test.BodyTable{
				"total": float64(2),
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
				"tokens.2.action": nil,
			},
		},
		{
			Name: "admin_list_register_tokens_with_status_filter",
			Request: &tokenmessage.ProfileTokens_Request{
				IsAdmin: true,
				Action:  tokenenum.Action_register,
				Status:  tokenenum.Status_done,
				SortBy:  "created_at:ASC",
			},
			Response: test.BodyTable{
				"total": float64(1),
				// ----
				"tokens.0.action":     float64(2),
				"tokens.0.status":     float64(1),
				"tokens.0.expired_at": "*",
				"tokens.0.updated_at": "*",
				"tokens.0.created_at": "*",
				// ----
				"tokens.1.action": nil,
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

	builder := factory.New(t, setup.Postgres.Conn().Conn)
	user := builder.Profile(factory.ProfileParams{Alias: "user", Name: "user", Surname: "user", Email: "user@werbot.net"})

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := tokenpb.NewTokenHandlersClient(setup)
		return a.AddTokenProfileReset(ctx, req.(*tokenmessage.AddTokenProfileReset_Request))
	}

	now := time.Now()

	testTable := []test.GRPCTable{
		{
			Name:    "missing_profile_id",
			Request: &tokenmessage.AddTokenProfileReset_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"profile_id": "value is required",
				},
			},
		},
		{
			Name: "invalid_profile_id_format",
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
			Name: "profile_not_found",
			Request: &tokenmessage.AddTokenProfileReset_Request{
				ProfileId: test.ConstFakeID,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Profile not found",
			},
		},
		{
			Name: "create_reset_token_with_custom_expiration",
			Request: &tokenmessage.AddTokenProfileReset_Request{
				ProfileId: user.ID,
				ExpiredAt: timestamppb.New(now.Add(36 * time.Hour)),
			},
			Response: test.BodyTable{
				"token": "*",
			},
		},
		{
			Name: "create_reset_token_with_past_expiration",
			Request: &tokenmessage.AddTokenProfileReset_Request{
				ProfileId: user.ID,
				ExpiredAt: timestamppb.New(now.Add(-1 * time.Hour)),
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
		{
			Name:    "missing_profile_data",
			Request: &tokenmessage.AddTokenProfileRegistration_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"data": "value is required",
				},
			},
		},
		{
			Name: "empty_profile_data_fields",
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
			Name: "invalid_profile_data_validation",
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
			Name: "create_registration_token_success",
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

	builder := factory.New(t, setup.Postgres.Conn().Conn)
	user := builder.Profile(factory.ProfileParams{Alias: "user", Name: "user", Surname: "user", Email: "user@werbot.net"})

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := tokenpb.NewTokenHandlersClient(setup)
		return a.AddTokenProfileDelete(ctx, req.(*tokenmessage.AddTokenProfileDelete_Request))
	}

	now := time.Now()

	testTable := []test.GRPCTable{
		{
			Name:    "missing_profile_id",
			Request: &tokenmessage.AddTokenProfileDelete_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"profile_id": "value is required",
				},
			},
		},
		{
			Name: "invalid_profile_id_format",
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
			Name: "profile_not_found",
			Request: &tokenmessage.AddTokenProfileDelete_Request{
				ProfileId: test.ConstFakeID,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Profile not found",
			},
		},
		{
			Name: "create_delete_token_success",
			Request: &tokenmessage.AddTokenProfileDelete_Request{
				ProfileId: user.ID,
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

	builder := factory.New(t, setup.Postgres.Conn().Conn)
	user := builder.Profile(factory.ProfileParams{Alias: "user", Name: "user", Surname: "user", Email: "user@werbot.net"})

	registerToken := builder.Token(factory.TokenParams{
		Section: 1,
		Action:  2,
		Status:  2,
		Data:    map[string]any{"name": "invite201", "surname": "invite201", "email": "invite201@werbot.net"},
	})

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := tokenpb.NewTokenHandlersClient(setup)
		return a.UpdateProfileToken(ctx, req.(*tokenmessage.UpdateProfileToken_Request))
	}

	testTable := []test.GRPCTable{
		{
			Name:    "missing_token_and_status",
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
			Name: "invalid_token_format_and_status_enum",
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
			Name: "missing_status",
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
			Name: "token_not_found",
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
			Name: "token_not_found_with_profile_id",
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
			Name: "register_token_missing_profile_id",
			Request: &tokenmessage.UpdateProfileToken_Request{
				Token:  registerToken.ID,
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
			Name: "register_token_profile_not_found",
			Request: &tokenmessage.UpdateProfileToken_Request{
				ProfileId: test.ConstFakeID,
				Token:     registerToken.ID,
				Status:    tokenenum.Status_done,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Profile not found",
			},
		},
		{
			Name: "update_register_token_to_done_success",
			Request: &tokenmessage.UpdateProfileToken_Request{
				ProfileId: user.ID,
				Token:     registerToken.ID,
				Status:    tokenenum.Status_done,
			},
			Response: test.BodyTable{},
		},
		{
			Name: "user_cannot_set_used_status",
			Request: &tokenmessage.UpdateProfileToken_Request{
				Token:  registerToken.ID,
				Status: tokenenum.Status_used,
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"status": "value must not be in list [0, 3]",
				},
			},
		},
		{
			Name: "user_cannot_set_deleted_status",
			Request: &tokenmessage.UpdateProfileToken_Request{
				Token:  registerToken.ID,
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
