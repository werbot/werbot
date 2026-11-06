package token_test

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	schemeaccesspb "github.com/werbot/werbot/internal/core/scheme/proto/access"
	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	tokenpb "github.com/werbot/werbot/internal/core/token/proto/rpc"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/internal/utils/test/factory"
	"github.com/werbot/werbot/pkg/uuid"
)

func Test_SchemeTokens(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	builder := factory.New(t, setup.Postgres.Conn().Conn)

	admin := builder.Profile(factory.ProfileParams{Alias: "admin", Role: 3})
	project := builder.Project(factory.ProjectParams{OwnerID: admin.ID, Title: "project1"})
	scheme := builder.Scheme(factory.SchemeParams{ProjectID: project.ID, SchemeType: 103, Title: "MySQL"})

	builder.Token(factory.TokenParams{
		Section:   3,
		Action:    3,
		Status:    2,
		ProjectID: project.ID,
		SchemeID:  scheme.ID,
		Data:      map[string]any{"email": "invite111@werbot.net", "scheme_type": 103},
		ExpiredAt: factory.Time(time.Now().UTC().Add(24 * time.Hour)),
	})

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := tokenpb.NewTokenHandlersClient(setup)
		return a.SchemeTokens(ctx, req.(*tokenmessage.SchemeTokens_Request))
	}

	testTable := []test.GRPCTable{
		{
			Name:    "missing_required_parameters",
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
			Name: "invalid_parameters_format",
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
			Name: "tokens_not_found_wrong_owner_and_scheme",
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
			Name: "tokens_not_found_wrong_scheme",
			Request: &tokenmessage.SchemeTokens_Request{
				OwnerId:  admin.ID,
				SchemeId: test.ConstFakeID,
				Action:   tokenenum.Action_add,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: trace.MsgTokenNotFound,
			},
		},
		{
			Name: "tokens_not_found_wrong_owner",
			Request: &tokenmessage.SchemeTokens_Request{
				OwnerId:  test.ConstFakeID,
				SchemeId: scheme.ID,
				Action:   tokenenum.Action_add,
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: trace.MsgTokenNotFound,
			},
		},
		{
			Name: "admin_list_scheme_tokens_success",
			Request: &tokenmessage.SchemeTokens_Request{
				OwnerId:  admin.ID,
				SchemeId: scheme.ID,
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
			Name: "admin_list_scheme_tokens_with_status_filter_no_results",
			Request: &tokenmessage.SchemeTokens_Request{
				OwnerId:  admin.ID,
				SchemeId: scheme.ID,
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

	builder := factory.New(t, setup.Postgres.Conn().Conn)

	admin := builder.Profile(factory.ProfileParams{Alias: "admin", Name: "admin", Surname: "admin", Email: "admin@werbot.net", Role: 3})
	user := builder.Profile(factory.ProfileParams{Alias: "user", Name: "user", Surname: "user", Email: "user@werbot.net"})
	project := builder.Project(factory.ProjectParams{OwnerID: admin.ID, Title: "project1"})

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := tokenpb.NewTokenHandlersClient(setup)
		return a.AddTokenSchemeAdd(ctx, req.(*tokenmessage.AddTokenSchemeAdd_Request))
	}

	now := time.Now()

	testTable := []test.GRPCTable{
		{
			Name:    "missing_required_parameters",
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
			Name: "missing_owner_and_project_with_email",
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
			Name: "invalid_project_id_format",
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
			Name: "invalid_email_format",
			Request: &tokenmessage.AddTokenSchemeAdd_Request{
				OwnerId:   admin.ID,
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
			Name: "project_not_found",
			Request: &tokenmessage.AddTokenSchemeAdd_Request{
				OwnerId:   admin.ID,
				ProjectId: test.ConstFakeID,
				Data: &tokenmessage.AddTokenSchemeAdd_Request_Email{
					Email: "admin@mail.com",
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Project not found",
			},
		},
		{
			Name: "project_not_found_wrong_owner",
			Request: &tokenmessage.AddTokenSchemeAdd_Request{
				OwnerId:   test.ConstFakeID,
				ProjectId: project.ID,
				Data: &tokenmessage.AddTokenSchemeAdd_Request_Email{
					Email: "admin@mail.com",
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Project not found",
			},
		},
		{
			Name: "create_scheme_add_token_with_email_success",
			Request: &tokenmessage.AddTokenSchemeAdd_Request{
				OwnerId:    admin.ID,
				ProjectId:  project.ID,
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
			Name: "create_scheme_add_token_with_email_duplicate",
			Request: &tokenmessage.AddTokenSchemeAdd_Request{
				OwnerId:    admin.ID,
				ProjectId:  project.ID,
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
			Name: "create_scheme_add_token_with_email_custom_expiration",
			Request: &tokenmessage.AddTokenSchemeAdd_Request{
				OwnerId:    admin.ID,
				ProjectId:  project.ID,
				SchemeType: schemeaccesspb.SchemeType_server_ssh,
				Data: &tokenmessage.AddTokenSchemeAdd_Request_Email{
					Email: "admin@werbot.net",
				},
				ExpiredAt: timestamppb.New(now.Add(36 * time.Hour)),
			},
			Response: test.BodyTable{
				"token": "*",
			},
		},
		{
			Name: "profile_not_found_with_profile_id",
			Request: &tokenmessage.AddTokenSchemeAdd_Request{
				OwnerId:    admin.ID,
				ProjectId:  project.ID,
				SchemeType: schemeaccesspb.SchemeType_server_ssh,
				Data: &tokenmessage.AddTokenSchemeAdd_Request_ProfileId{
					ProfileId: test.ConstFakeID,
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Profile not found",
			},
		},
		{
			Name: "invalid_profile_id_format",
			Request: &tokenmessage.AddTokenSchemeAdd_Request{
				OwnerId:    admin.ID,
				ProjectId:  project.ID,
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
		{
			Name: "create_scheme_add_token_with_profile_id_success",
			Request: &tokenmessage.AddTokenSchemeAdd_Request{
				OwnerId:    admin.ID,
				ProjectId:  project.ID,
				SchemeType: schemeaccesspb.SchemeType_server_ssh,
				Data: &tokenmessage.AddTokenSchemeAdd_Request_ProfileId{
					ProfileId: user.ID,
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

	builder := factory.New(t, setup.Postgres.Conn().Conn)

	admin := builder.Profile(factory.ProfileParams{Alias: "admin", Name: "admin", Surname: "admin", Email: "admin@werbot.net", Role: 3})
	user := builder.Profile(factory.ProfileParams{Alias: "user", Name: "user", Surname: "user", Email: "user@werbot.net"})
	project := builder.Project(factory.ProjectParams{OwnerID: admin.ID, Title: "project1"})
	scheme := builder.Scheme(factory.SchemeParams{ProjectID: project.ID, SchemeType: 103, Title: "MySQL"})

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := tokenpb.NewTokenHandlersClient(setup)
		return a.AddTokenSchemeAccess(ctx, req.(*tokenmessage.AddTokenSchemeAccess_Request))
	}

	now := time.Now()

	testTable := []test.GRPCTable{
		{
			Name:    "missing_required_parameters",
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
			Name: "missing_owner_and_scheme_with_email",
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
			Name: "invalid_parameters_format_with_email",
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
			Name: "invalid_email_format",
			Request: &tokenmessage.AddTokenSchemeAccess_Request{
				OwnerId:  admin.ID,
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
			Name: "scheme_not_found",
			Request: &tokenmessage.AddTokenSchemeAccess_Request{
				OwnerId:  admin.ID,
				SchemeId: test.ConstFakeID,
				Data: &tokenmessage.AddTokenSchemeAccess_Request_Email{
					Email: "admin@mail.com",
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Scheme not found",
			},
		},
		{
			Name: "scheme_not_found_wrong_owner",
			Request: &tokenmessage.AddTokenSchemeAccess_Request{
				OwnerId:  test.ConstFakeID,
				SchemeId: scheme.ID,
				Data: &tokenmessage.AddTokenSchemeAccess_Request_Email{
					Email: "user@mail.com",
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Scheme not found",
			},
		},
		{
			Name: "create_scheme_access_token_with_email_success",
			Request: &tokenmessage.AddTokenSchemeAccess_Request{
				OwnerId:  admin.ID,
				SchemeId: scheme.ID,
				Data: &tokenmessage.AddTokenSchemeAccess_Request_Email{
					Email: "user@mail.com",
				},
			},
			Response: test.BodyTable{
				"token": "*",
			},
		},
		{
			Name: "profile_not_found_with_profile_id",
			Request: &tokenmessage.AddTokenSchemeAccess_Request{
				OwnerId:  admin.ID,
				SchemeId: scheme.ID,
				Data: &tokenmessage.AddTokenSchemeAccess_Request_ProfileId{
					ProfileId: test.ConstFakeID,
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Profile not found",
			},
		},
		{
			Name: "create_scheme_access_token_with_profile_id_success",
			Request: &tokenmessage.AddTokenSchemeAccess_Request{
				OwnerId:  admin.ID,
				SchemeId: scheme.ID,
				Data: &tokenmessage.AddTokenSchemeAccess_Request_ProfileId{
					ProfileId: user.ID,
				},
			},
			Response: test.BodyTable{
				"token": "*",
			},
		},
		{
			Name: "create_scheme_access_token_with_profile_id_custom_expiration",
			Request: &tokenmessage.AddTokenSchemeAccess_Request{
				OwnerId:  admin.ID,
				SchemeId: scheme.ID,
				Data: &tokenmessage.AddTokenSchemeAccess_Request_ProfileId{
					ProfileId: user.ID,
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
		{
			Name:    "missing_token_and_status",
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
			Name: "invalid_token_format_and_status_enum",
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
			Name: "missing_status",
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
			Name: "token_not_found",
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
			Name: "token_not_found_duplicate",
			Request: &tokenmessage.UpdateSchemeToken_Request{
				Token:  test.ConstFakeID,
				Status: tokenenum.Status_done,
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

type connectFixture struct {
	setup        *test.GRPCHandler
	builder      *factory.Builder
	client       tokenpb.TokenHandlersClient
	admin, other factory.Profile
	project      factory.Project
	scheme       factory.Scheme
	schemeMember factory.SchemeMember
}

func newConnectFixture(t *testing.T) *connectFixture {
	t.Helper()

	setup, teardown := test.GRPC(t)
	t.Cleanup(func() { teardown(t) })

	builder := factory.New(t, setup.Postgres.Conn().Conn)
	admin := builder.Profile(factory.ProfileParams{Alias: "admin", Role: 3})
	other := builder.Profile(factory.ProfileParams{Alias: "other"})
	project := builder.Project(factory.ProjectParams{OwnerID: admin.ID, Title: "project1"})
	member := builder.ProjectMember(factory.ProjectMemberParams{ProjectID: project.ID, ProfileID: admin.ID})
	scheme := builder.Scheme(factory.SchemeParams{ProjectID: project.ID, SchemeType: 104, Title: "telnet"})
	schemeMember := builder.SchemeMember(factory.SchemeMemberParams{SchemeID: scheme.ID, ProjectMemberID: member.ID})

	return &connectFixture{
		setup:        setup,
		builder:      builder,
		client:       tokenpb.NewTokenHandlersClient(setup),
		admin:        admin,
		other:        other,
		project:      project,
		scheme:       scheme,
		schemeMember: schemeMember,
	}
}

func Test_AddTokenSchemeConnect(t *testing.T) {
	t.Run("missing_required_parameters", func(t *testing.T) {
		f := newConnectFixture(t)
		_, err := f.client.AddTokenSchemeConnect(context.Background(), &tokenmessage.AddTokenSchemeConnect_Request{})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
		}
	})

	t.Run("member_success", func(t *testing.T) {
		f := newConnectFixture(t)
		now := time.Now()

		response, err := f.client.AddTokenSchemeConnect(context.Background(), &tokenmessage.AddTokenSchemeConnect_Request{
			ProfileId: f.admin.ID,
			SchemeId:  f.scheme.ID,
		})
		if err != nil {
			t.Fatalf("AddTokenSchemeConnect() error = %v", err)
		}
		if !uuid.IsValid(response.GetToken()) {
			t.Fatalf("token = %q, want UUID", response.GetToken())
		}
		expired := response.GetExpiredAt().AsTime()
		if expired.Before(now.Add(4*time.Minute+59*time.Second)) || expired.After(now.Add(5*time.Minute+time.Second)) {
			t.Fatalf("expired_at = %v, want about now+5m", expired)
		}

		var section, action, status int16
		var profileID, schemeID string
		var expiredAt time.Time
		err = f.setup.Postgres.Conn().Conn.QueryRow(`
			SELECT "section", "action", "status", "profile_id", "scheme_id", "expired_at"
			FROM "token" WHERE "id" = $1
		`, response.GetToken()).Scan(&section, &action, &status, &profileID, &schemeID, &expiredAt)
		if err != nil {
			t.Fatalf("query token: %v", err)
		}
		if section != int16(tokenenum.Section_scheme) || action != int16(tokenenum.Action_connect) || status != int16(tokenenum.Status_sent) {
			t.Fatalf("token row = section %d action %d status %d", section, action, status)
		}
		if profileID != f.admin.ID || schemeID != f.scheme.ID {
			t.Fatalf("token row = profile %q scheme %q", profileID, schemeID)
		}
	})

	t.Run("not_member", func(t *testing.T) {
		f := newConnectFixture(t)
		_, err := f.client.AddTokenSchemeConnect(context.Background(), &tokenmessage.AddTokenSchemeConnect_Request{
			ProfileId: f.other.ID,
			SchemeId:  f.scheme.ID,
		})
		if status.Code(err) != codes.NotFound || status.Convert(err).Message() != trace.MsgSchemeNotFound {
			t.Fatalf("error = %v, want NotFound %q", err, trace.MsgSchemeNotFound)
		}
	})

	t.Run("inactive_scheme_member", func(t *testing.T) {
		f := newConnectFixture(t)
		if _, err := f.setup.Postgres.Conn().Conn.Exec(`UPDATE "scheme_member" SET "active" = FALSE WHERE "id" = $1`, f.schemeMember.ID); err != nil {
			t.Fatalf("deactivate scheme member: %v", err)
		}
		_, err := f.client.AddTokenSchemeConnect(context.Background(), &tokenmessage.AddTokenSchemeConnect_Request{
			ProfileId: f.admin.ID,
			SchemeId:  f.scheme.ID,
		})
		if status.Code(err) != codes.NotFound || status.Convert(err).Message() != trace.MsgSchemeNotFound {
			t.Fatalf("error = %v, want NotFound %q", err, trace.MsgSchemeNotFound)
		}
	})
}

func Test_UseTokenSchemeConnect(t *testing.T) {
	f := newConnectFixture(t)
	ctx := context.Background()

	add := func(t *testing.T) *tokenmessage.AddTokenSchemeConnect_Response {
		t.Helper()
		response, err := f.client.AddTokenSchemeConnect(ctx, &tokenmessage.AddTokenSchemeConnect_Request{
			ProfileId: f.admin.ID,
			SchemeId:  f.scheme.ID,
		})
		if err != nil {
			t.Fatalf("AddTokenSchemeConnect() error = %v", err)
		}
		return response
	}

	t.Run("success", func(t *testing.T) {
		token := add(t)
		response, err := f.client.UseTokenSchemeConnect(ctx, &tokenmessage.UseTokenSchemeConnect_Request{Token: token.GetToken()})
		if err != nil {
			t.Fatalf("UseTokenSchemeConnect() error = %v", err)
		}
		if response.GetProfileId() != f.admin.ID || response.GetSchemeId() != f.scheme.ID || response.GetSchemeMemberId() != f.schemeMember.ID {
			t.Fatalf("response = %+v", response)
		}

		var statusValue int16
		if err := f.setup.Postgres.Conn().Conn.QueryRow(`SELECT "status" FROM "token" WHERE "id" = $1`, token.GetToken()).Scan(&statusValue); err != nil {
			t.Fatalf("query token status: %v", err)
		}
		if statusValue != int16(tokenenum.Status_used) {
			t.Fatalf("status = %d, want used", statusValue)
		}
	})

	t.Run("second_use", func(t *testing.T) {
		token := add(t)
		if _, err := f.client.UseTokenSchemeConnect(ctx, &tokenmessage.UseTokenSchemeConnect_Request{Token: token.GetToken()}); err != nil {
			t.Fatalf("first UseTokenSchemeConnect() error = %v", err)
		}
		_, err := f.client.UseTokenSchemeConnect(ctx, &tokenmessage.UseTokenSchemeConnect_Request{Token: token.GetToken()})
		if status.Code(err) != codes.NotFound || status.Convert(err).Message() != trace.MsgTokenIsInvalid {
			t.Fatalf("error = %v, want NotFound %q", err, trace.MsgTokenIsInvalid)
		}
	})

	t.Run("expired", func(t *testing.T) {
		token := f.builder.Token(factory.TokenParams{
			Section:   int16(tokenenum.Section_scheme),
			Action:    int16(tokenenum.Action_connect),
			Status:    int16(tokenenum.Status_sent),
			ProfileID: f.admin.ID,
			SchemeID:  f.scheme.ID,
			ExpiredAt: factory.Time(time.Now().UTC().Add(-time.Minute)),
		})
		_, err := f.client.UseTokenSchemeConnect(ctx, &tokenmessage.UseTokenSchemeConnect_Request{Token: token.ID})
		if status.Code(err) != codes.NotFound || status.Convert(err).Message() != trace.MsgTokenIsInvalid {
			t.Fatalf("error = %v, want NotFound %q", err, trace.MsgTokenIsInvalid)
		}
	})

	t.Run("wrong_action", func(t *testing.T) {
		token := f.builder.Token(factory.TokenParams{
			Section:   int16(tokenenum.Section_scheme),
			Action:    int16(tokenenum.Action_access),
			Status:    int16(tokenenum.Status_sent),
			ProfileID: f.admin.ID,
			SchemeID:  f.scheme.ID,
		})
		_, err := f.client.UseTokenSchemeConnect(ctx, &tokenmessage.UseTokenSchemeConnect_Request{Token: token.ID})
		if status.Code(err) != codes.NotFound || status.Convert(err).Message() != trace.MsgTokenIsInvalid {
			t.Fatalf("error = %v, want NotFound %q", err, trace.MsgTokenIsInvalid)
		}
	})

	t.Run("membership_revoked", func(t *testing.T) {
		token := add(t)
		if _, err := f.setup.Postgres.Conn().Conn.Exec(`UPDATE "scheme_member" SET "active" = FALSE WHERE "id" = $1`, f.schemeMember.ID); err != nil {
			t.Fatalf("deactivate scheme member: %v", err)
		}
		_, err := f.client.UseTokenSchemeConnect(ctx, &tokenmessage.UseTokenSchemeConnect_Request{Token: token.GetToken()})
		if status.Code(err) != codes.NotFound || status.Convert(err).Message() != trace.MsgTokenIsInvalid {
			t.Fatalf("error = %v, want NotFound %q", err, trace.MsgTokenIsInvalid)
		}
	})
}

func Test_AddTokenSchemeConnectProjectOwner(t *testing.T) {
	setup, teardown := test.GRPC(t)
	defer teardown(t)

	builder := factory.New(t, setup.Postgres.Conn().Conn)
	owner := builder.Profile(factory.ProfileParams{Alias: "owner", Role: 3})
	project := builder.Project(factory.ProjectParams{OwnerID: owner.ID, Title: "owner project"})
	scheme := builder.Scheme(factory.SchemeParams{ProjectID: project.ID, SchemeType: 104, Title: "telnet"})
	client := tokenpb.NewTokenHandlersClient(setup)
	ctx := context.Background()

	response, err := client.AddTokenSchemeConnect(ctx, &tokenmessage.AddTokenSchemeConnect_Request{
		ProfileId: owner.ID,
		SchemeId:  scheme.ID,
	})
	if err != nil {
		t.Fatalf("AddTokenSchemeConnect() error = %v", err)
	}

	var projectMemberID, schemeMemberID string
	err = setup.Postgres.Conn().Conn.QueryRow(`
		SELECT pm.id, sm.id
		FROM project_member pm
		INNER JOIN scheme_member sm ON sm.project_member_id = pm.id
		WHERE pm.project_id = $1 AND pm.profile_id = $2 AND pm.active = TRUE AND sm.active = TRUE
	`, project.ID, owner.ID).Scan(&projectMemberID, &schemeMemberID)
	if err != nil {
		t.Fatalf("owner membership was not created: %v", err)
	}

	used, err := client.UseTokenSchemeConnect(ctx, &tokenmessage.UseTokenSchemeConnect_Request{Token: response.GetToken()})
	if err != nil {
		t.Fatalf("UseTokenSchemeConnect() error = %v", err)
	}
	if used.GetSchemeMemberId() != schemeMemberID {
		t.Fatalf("scheme_member_id = %q, want %q", used.GetSchemeMemberId(), schemeMemberID)
	}
}
