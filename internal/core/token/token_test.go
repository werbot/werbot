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
		{ // request without parameters
			Name:    "test0_01",
			Request: &tokenmessage.Token_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"token": "value is required",
				},
			},
		},
		{
			Name: "test0_02",
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
			Name: "test0_03",
			Request: &tokenmessage.Token_Request{
				Token: test.ConstFakeID,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: trace.MsgTokenNotFound,
			},
		},

		// PROFILE section
		{ // admin mode, data not null
			Name: "test1_01",
			Request: &tokenmessage.Token_Request{
				IsAdmin: true,
				Token:   "3c818d7c-72f3-4518-8eaa-755585192f21",
			},
			Response: test.BodyTable{
				"owner_id":        "b3dc36e2-7f84-414b-b147-7ac850369518",
				"section":         float64(1),
				"action":          float64(2),
				"status":          float64(1),
				"profile_id":      "*", // visible in admin mode
				"profile.name":    "Harrison",
				"profile.surname": "Bowling",
				"profile.email":   "user1@werbot.net",
				"expired_at":      "*",
				"updated_at":      "*",
				"created_at":      "*",
			},
		},
		{ // data not null
			Name: "test1_02",
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
		{ // data is null
			Name: "test1_03",
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

		// PROJECT section
		{ // admin mode
			Name: "test2_01",
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
		{ // user mode
			Name: "test2_02",
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

		// SCHEME section
		{ // admin mode
			Name: "test3_01",
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
		{ // user mode
			Name: "test3_02",
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

		// AGENT section
		{ // admin mode
			Name: "test4_01",
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
		{ // user mode
			Name: "test4_02",
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

/*
func Test_UpdateToken(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := tokenpb.NewTokenHandlersClient(setup)
		return a.UpdateToken(ctx, req.(*tokenmessage.UpdateToken_Request))
	}

	testTable := []test.GRPCTable{
		{ // request without parameters
			Name:    "test0_01",
			Request: &tokenmessage.UpdateToken_Request{},
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
			Request: &tokenmessage.UpdateToken_Request{
				OwnerId: "ok",
				Token:   "ok",
				Status:  999,
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id": "value must be a valid UUID",
					"token":    "value must be a valid UUID",
					"status":   "value must be one of the defined enum values",
				},
			},
		},
		{
			Name: "test0_03",
			Request: &tokenmessage.UpdateToken_Request{
				OwnerId: test.ConstFakeID,
				Token:   test.ConstFakeID,
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
			Request: &tokenmessage.UpdateToken_Request{
				OwnerId: test.ConstFakeID,
				Token:   test.ConstFakeID,
				Status:  tokenenum.Status_done,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: trace.MsgTokenNotFound,
			},
		},

		// PROFILE setion
		{ // 1-2-1, ERROR
			Name: "test1_01",
			Request: &tokenmessage.UpdateToken_Request{
				Token:     test.ConstFakeID,
				Status:    tokenenum.Status_done,
				ProfileId: test.ConstFakeID,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: trace.MsgTokenNotFound,
			},
		},
		{ // 1-2-1, ERROR
			Name: "test1_02",
			Request: &tokenmessage.UpdateToken_Request{
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
		{ // 1-2-1, ERROR
			Name: "test1_03",
			Request: &tokenmessage.UpdateToken_Request{
				ProfileId: test.ConstFakeID,
				Token:     "55c9f79c-d827-43fc-8ad1-79e396d2432c",
				Status:    tokenenum.Status_done,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Profile not found",
			},
		},
		{ // 1-2-1, DONE
			Name: "test1_04",
			Request: &tokenmessage.UpdateToken_Request{
				ProfileId: "5a2bb353-e862-4e6f-be74-fedb2dd761fd",
				Token:     "55c9f79c-d827-43fc-8ad1-79e396d2432c",
				Status:    tokenenum.Status_done,
			},
			Response: test.BodyTable{},
		},
		{ // 1-2-3, DONE
			Name: "test1_05",
			Request: &tokenmessage.UpdateToken_Request{
				Token:  "55c9f79c-d827-43fc-8ad1-79e396d2432c",
				Status: tokenenum.Status_used,
			},
			Response: test.BodyTable{},
		},
		{ // 1-2-4, ERROR
			Name: "test1_06",
			Request: &tokenmessage.UpdateToken_Request{
				Token:  "55c9f79c-d827-43fc-8ad1-79e396d2432c",
				Status: tokenenum.Status_deleted,
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"status": "value must not be in list [0, 4]",
				},
			},
		},

		// PROJECT setion
		{ // 2-2-1, ERROR
			Name: "test2_01",
			Request: &tokenmessage.UpdateToken_Request{
				Token:     test.ConstFakeID,
				Status:    tokenenum.Status_done,
				ProjectId: test.ConstFakeID,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: trace.MsgTokenNotFound,
			},
		},
		{ // 2-2-1, ERROR
			Name: "test2_02",
			Request: &tokenmessage.UpdateToken_Request{
				Token:  "b5b128d7-ff0c-479c-8216-14eedf9265ad",
				Status: tokenenum.Status_done,
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id": "value is required",
				},
			},
		},
		{ // 2-2-1, ERROR
			Name: "test2_03",
			Request: &tokenmessage.UpdateToken_Request{
				OwnerId: test.ConstFakeID,
				Token:   "b5b128d7-ff0c-479c-8216-14eedf9265ad",
				Status:  tokenenum.Status_done,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Owner not found",
			},
		},
		{ // 2-2-1, DONE
			Name: "test2_04",
			Request: &tokenmessage.UpdateToken_Request{
				OwnerId: test.ConstAdminID,
				Token:   "b5b128d7-ff0c-479c-8216-14eedf9265ad",
				Status:  tokenenum.Status_done,
			},
			Response: test.BodyTable{},
		},
		{ // 2-2-2, DONE
			Name: "test2_05",
			Request: &tokenmessage.UpdateToken_Request{
				OwnerId: test.ConstAdminID,
				Token:   "b5b128d7-ff0c-479c-8216-14eedf9265ad",
				Status:  tokenenum.Status_done,
			},
			Response: test.BodyTable{},
		},
		{ // 2-2-3, DONE
			Name: "test2_06",
			Request: &tokenmessage.UpdateToken_Request{
				OwnerId: test.ConstAdminID,
				Token:   "b5b128d7-ff0c-479c-8216-14eedf9265ad",
				Status:  tokenenum.Status_used,
			},
			Response: test.BodyTable{},
		},
		{ // 2-2-4, ERROR
			Name: "test2_07",
			Request: &tokenmessage.UpdateToken_Request{
				OwnerId: test.ConstAdminID,
				Token:   "b5b128d7-ff0c-479c-8216-14eedf9265ad",
				Status:  tokenenum.Status_deleted,
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"status": "value must not be in list [0, 4]",
				},
			},
		},
		{ // 2-3-1, ERROR
			Name: "test2_08",
			Request: &tokenmessage.UpdateToken_Request{
				OwnerId: test.ConstAdminID,
				Token:   "f0e9381e-ed3c-431c-bca2-019f8712436f",
				Status:  tokenenum.Status_done,
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"profile_id": "value is required",
				},
			},
		},
		{ // 2-3-1, ERROR
			Name: "test2_09",
			Request: &tokenmessage.UpdateToken_Request{
				OwnerId:   test.ConstAdminID,
				Token:     "f0e9381e-ed3c-431c-bca2-019f8712436f",
				Status:    tokenenum.Status_done,
				ProfileId: test.ConstFakeID,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Profile not found",
			},
		},
		{ // 2-3-1, DONE
			Name: "test2_10",
			Request: &tokenmessage.UpdateToken_Request{
				OwnerId:   test.ConstAdminID,
				Token:     "f0e9381e-ed3c-431c-bca2-019f8712436f",
				Status:    tokenenum.Status_done,
				ProfileId: test.ConstUserID,
			},
			Response: test.BodyTable{},
		},

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

		// AGENT setion
		{ // 4-3-1, ERROR
			Name: "test4_01",
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
		{ // 4-3-1, ERROR
			Name: "test4_02",
			Request: &tokenmessage.UpdateToken_Request{
				Token:  "0a177fc3-ad38-40c6-b936-ded649ce5a57",
				Status: tokenenum.Status_done,
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"scheme_id": "value is required",
				},
			},
		},
		{ // 4-3-1, DONE
			Name: "test4_03",
			Request: &tokenmessage.UpdateToken_Request{
				Token:    "0a177fc3-ad38-40c6-b936-ded649ce5a57",
				Status:   tokenenum.Status_done,
				SchemeId: "df2046ee-5932-437f-b825-e55399666e45",
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
*/

func Test_DeleteToken(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := tokenpb.NewTokenHandlersClient(setup)
		return a.DeleteToken(ctx, req.(*tokenmessage.DeleteToken_Request))
	}

	testTable := []test.GRPCTable{
		{ // request without parameters
			Name:    "test0_01",
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
			Name: "test0_02",
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
			Name: "test0_03",
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
			Name: "test0_04",
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
			Name: "test0_05",
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
			Name: "test0_06",
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
			Name: "test0_07",
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
