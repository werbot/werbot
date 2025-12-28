package token_test

import (
	"context"
	"testing"

	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	tokenpb "github.com/werbot/werbot/internal/core/token/proto/rpc"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/internal/utils/test"
	"google.golang.org/grpc/codes"
)

func Test_Token(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := tokenpb.NewTokenHandlersClient(setup)
		return a.Token(ctx, req.(*tokenmessage.Token_Request))
	}

	testTable := []test.GRPCTable{
		{
			Name:    "missing_token_parameter",
			Request: &tokenmessage.Token_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"token": "value is required",
				},
			},
		},
		{
			Name: "invalid_token_format",
			Request: &tokenmessage.Token_Request{
				Token: "ok",
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"token": "value must be a valid UUID",
				},
			},
		},
		{
			Name: "token_not_found",
			Request: &tokenmessage.Token_Request{
				Token: test.ConstFakeID,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: trace.MsgTokenNotFound,
			},
		},
		{
			Name: "admin_get_profile_token_with_metadata",
			Request: &tokenmessage.Token_Request{
				IsAdmin: true,
				Token:   "3c818d7c-72f3-4518-8eaa-755585192f21",
			},
			Response: test.BodyTable{
				"owner_id":        "b3dc36e2-7f84-414b-b147-7ac850369518",
				"section":         float64(1),
				"action":          float64(2),
				"status":          float64(1),
				"profile_id":      "*",
				"profile.name":    "Harrison",
				"profile.surname": "Bowling",
				"profile.email":   "user1@werbot.net",
				"expired_at":      "*",
				"updated_at":      "*",
				"created_at":      "*",
			},
		},
		{
			Name: "user_get_profile_token_masked",
			Request: &tokenmessage.Token_Request{
				Token: "3c818d7c-72f3-4518-8eaa-755585192f21",
			},
			Response: test.BodyTable{
				"owner_id":        nil,
				"section":         float64(1),
				"action":          float64(2),
				"status":          float64(1),
				"profile_id":      nil,
				"profile.name":    "Harrison",
				"profile.surname": "Bowling",
				"profile.email":   "user1@werbot.net",
				"expired_at":      "*",
				"updated_at":      "*",
				"created_at":      "*",
			},
		},
		{
			Name: "user_get_profile_token_without_metadata",
			Request: &tokenmessage.Token_Request{
				Token: "374060a3-6c9c-44b5-9fbd-ce1ea8af6c7b",
			},
			Response: test.BodyTable{
				"owner_id":   nil,
				"section":    float64(1),
				"action":     float64(4),
				"status":     float64(2),
				"profile_id": nil,
				"profile":    map[string]any{},
				"expired_at": "*",
				"updated_at": nil,
				"created_at": "*",
			},
		},
		{
			Name: "admin_get_project_token_with_metadata",
			Request: &tokenmessage.Token_Request{
				IsAdmin: true,
				Token:   "04439feb-f981-4581-8f2c-96f21418f258",
			},
			Response: test.BodyTable{
				"owner_id":        "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				"section":         float64(2),
				"action":          float64(3),
				"status":          float64(2),
				"project_id":      "*",
				"profile.name":    "invite100",
				"profile.surname": "invite100",
				"profile.email":   "invite100@werbot.net",
				"expired_at":      "*",
				"updated_at":      "*",
				"created_at":      "*",
			},
		},
		{
			Name: "user_get_project_token_masked",
			Request: &tokenmessage.Token_Request{
				Token: "04439feb-f981-4581-8f2c-96f21418f258",
			},
			Response: test.BodyTable{
				"owner_id":        nil,
				"section":         float64(2),
				"action":          float64(3),
				"status":          float64(2),
				"project_id":      nil,
				"profile.name":    "invite100",
				"profile.surname": "invite100",
				"profile.email":   "invite100@werbot.net",
				"expired_at":      "*",
				"updated_at":      "*",
				"created_at":      "*",
			},
		},
		{
			Name: "admin_get_scheme_token_with_metadata",
			Request: &tokenmessage.Token_Request{
				IsAdmin: true,
				Token:   "0a7b333e-4c98-4edd-9e34-a4e734a5926e",
			},
			Response: test.BodyTable{
				"owner_id":           "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				"section":            float64(3),
				"action":             float64(3),
				"status":             float64(2),
				"scheme_id":          "*",
				"scheme.scheme_type": float64(103),
				"scheme.email":       "invite109@werbot.net",
				"expired_at":         "*",
				"updated_at":         nil,
				"created_at":         "*",
			},
		},
		{
			Name: "user_get_scheme_token_masked",
			Request: &tokenmessage.Token_Request{
				Token: "0a7b333e-4c98-4edd-9e34-a4e734a5926e",
			},
			Response: test.BodyTable{
				"owner_id":           nil,
				"section":            float64(3),
				"action":             float64(3),
				"status":             float64(2),
				"scheme_id":          nil,
				"scheme.scheme_type": float64(103),
				"scheme.email":       "invite109@werbot.net",
				"expired_at":         "*",
				"updated_at":         nil,
				"created_at":         "*",
			},
		},
		{
			Name: "admin_get_agent_token_with_metadata",
			Request: &tokenmessage.Token_Request{
				IsAdmin: true,
				Token:   "39c45364-4b94-45bb-88b4-1360245f8a59",
			},
			Response: test.BodyTable{
				"owner_id":          "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				"section":           float64(4),
				"action":            float64(3),
				"status":            float64(2),
				"project_id":        "*",
				"agent.scheme_type": float64(103),
				"expired_at":        "*",
				"updated_at":        nil,
				"created_at":        "*",
			},
		},
		{
			Name: "user_get_agent_token_masked",
			Request: &tokenmessage.Token_Request{
				Token: "39c45364-4b94-45bb-88b4-1360245f8a59",
			},
			Response: test.BodyTable{
				"owner_id":          nil,
				"section":           float64(4),
				"action":            float64(3),
				"status":            float64(2),
				"project_id":        nil,
				"agent.scheme_type": float64(103),
				"expired_at":        "*",
				"updated_at":        nil,
				"created_at":        "*",
			},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			test.RunCaseGRPCTests(t, handler, tt)
		})
	}
}

func Test_DeleteToken(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := tokenpb.NewTokenHandlersClient(setup)
		return a.DeleteToken(ctx, req.(*tokenmessage.DeleteToken_Request))
	}

	testTable := []test.GRPCTable{
		{
			Name:    "missing_owner_id_and_token",
			Request: &tokenmessage.DeleteToken_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id": "value is required",
					"token":    "value is required",
				},
			},
		},
		{
			Name: "invalid_token_format",
			Request: &tokenmessage.DeleteToken_Request{
				Token: "ok",
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id": "value is required",
					"token":    "value must be a valid UUID",
				},
			},
		},
		{
			Name: "invalid_owner_id_format",
			Request: &tokenmessage.DeleteToken_Request{
				OwnerId: "ok",
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id": "value must be a valid UUID",
					"token":    "value is required",
				},
			},
		},
		{
			Name: "token_not_found_with_fake_ids",
			Request: &tokenmessage.DeleteToken_Request{
				OwnerId: test.ConstFakeID,
				Token:   test.ConstFakeID,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: trace.MsgTokenNotFound,
			},
		},
		{
			Name: "token_not_found_wrong_owner",
			Request: &tokenmessage.DeleteToken_Request{
				OwnerId: test.ConstFakeID,
				Token:   "3c818d7c-72f3-4518-8eaa-755585192f21",
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: trace.MsgTokenNotFound,
			},
		},
		{
			Name: "token_not_found_admin_wrong_token",
			Request: &tokenmessage.DeleteToken_Request{
				OwnerId: test.ConstAdminID,
				Token:   "3c818d7c-72f3-4518-8eaa-755585192f21",
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: trace.MsgTokenNotFound,
			},
		},
		{
			Name: "admin_delete_token_success",
			Request: &tokenmessage.DeleteToken_Request{
				OwnerId: test.ConstAdminID,
				Token:   "37aec639-dd1c-4c73-a8e7-add2016050f7",
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
