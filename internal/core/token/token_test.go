package token_test

import (
	"context"
	"testing"
	"time"

	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	tokenpb "github.com/werbot/werbot/internal/core/token/proto/rpc"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/internal/utils/test/factory"
	"google.golang.org/grpc/codes"
)

func Test_Token(t *testing.T) {
	setup, teardownTestCase := test.GRPCWithoutFixtures(t)
	defer teardownTestCase(t)

	builder := factory.New(t, setup.Postgres.Conn().Conn)

	admin := builder.Profile(factory.ProfileParams{Alias: "admin", Role: 3})
	user1 := builder.Profile(factory.ProfileParams{
		Alias:   "user1",
		Name:    "Harrison",
		Surname: "Bowling",
		Email:   "user1@werbot.net",
	})
	project := builder.Project(factory.ProjectParams{OwnerID: admin.ID, Title: "project1"})
	scheme := builder.Scheme(factory.SchemeParams{ProjectID: project.ID, SchemeType: 103, Title: "MySQL"})

	expiredAt := factory.Time(time.Now().UTC().Add(24 * time.Hour))
	updatedAt := factory.Time(time.Now().UTC())

	profileToken := builder.Token(factory.TokenParams{
		Section:   1,
		Action:    2,
		Status:    1,
		ProfileID: user1.ID,
		Data: map[string]any{
			"name":    "Harrison",
			"surname": "Bowling",
			"email":   "user1@werbot.net",
		},
		ExpiredAt: expiredAt,
		UpdatedAt: updatedAt,
	})
	deleteToken := builder.Token(factory.TokenParams{
		Section:   1,
		Action:    4,
		Status:    2,
		ExpiredAt: expiredAt,
	})
	projectToken := builder.Token(factory.TokenParams{
		Section:   2,
		Action:    3,
		Status:    2,
		ProjectID: project.ID,
		Data: map[string]any{
			"name":    "invite100",
			"surname": "invite100",
			"email":   "invite100@werbot.net",
		},
		ExpiredAt: expiredAt,
		UpdatedAt: updatedAt,
	})
	schemeToken := builder.Token(factory.TokenParams{
		Section:   3,
		Action:    3,
		Status:    2,
		ProjectID: project.ID,
		SchemeID:  scheme.ID,
		Data: map[string]any{
			"scheme_type": 103,
			"email":       "invite109@werbot.net",
		},
		ExpiredAt: expiredAt,
	})
	agentToken := builder.Token(factory.TokenParams{
		Section:   4,
		Action:    3,
		Status:    2,
		ProjectID: project.ID,
		Data:      map[string]any{"scheme_type": 103},
		ExpiredAt: expiredAt,
	})

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
				Token:   profileToken.ID,
			},
			Response: test.BodyTable{
				"owner_id":        user1.ID,
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
				Token: profileToken.ID,
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
				Token: deleteToken.ID,
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
				Token:   projectToken.ID,
			},
			Response: test.BodyTable{
				"owner_id":        admin.ID,
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
				Token: projectToken.ID,
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
				Token:   schemeToken.ID,
			},
			Response: test.BodyTable{
				"owner_id":           admin.ID,
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
				Token: schemeToken.ID,
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
				Token:   agentToken.ID,
			},
			Response: test.BodyTable{
				"owner_id":          admin.ID,
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
				Token: agentToken.ID,
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
	setup, teardownTestCase := test.GRPCWithoutFixtures(t)
	defer teardownTestCase(t)

	builder := factory.New(t, setup.Postgres.Conn().Conn)

	admin := builder.Profile(factory.ProfileParams{Alias: "admin", Role: 3})
	user1 := builder.Profile(factory.ProfileParams{
		Alias:   "user1",
		Name:    "Harrison",
		Surname: "Bowling",
		Email:   "user1@werbot.net",
	})

	userToken := builder.Token(factory.TokenParams{
		Section:   1,
		Action:    2,
		Status:    1,
		ProfileID: user1.ID,
	})
	adminToken := builder.Token(factory.TokenParams{
		Section:   1,
		Action:    2,
		Status:    2,
		ProfileID: admin.ID,
	})

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
					"token": "value is required",
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
					"token": "value must be a valid UUID",
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
				Token:   userToken.ID,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: trace.MsgTokenNotFound,
			},
		},
		{
			Name: "token_not_found_admin_wrong_token",
			Request: &tokenmessage.DeleteToken_Request{
				OwnerId: admin.ID,
				Token:   userToken.ID,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: trace.MsgTokenNotFound,
			},
		},
		{
			Name: "admin_delete_token_success",
			Request: &tokenmessage.DeleteToken_Request{
				OwnerId: admin.ID,
				Token:   adminToken.ID,
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
