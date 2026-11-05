package audit_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	auditmessage "github.com/werbot/werbot/internal/core/audit/proto/message"
	auditrpc "github.com/werbot/werbot/internal/core/audit/proto/rpc"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/internal/utils/test/factory"
	"github.com/werbot/werbot/pkg/uuid"
)

// unknownID points at no row created by the test.
var unknownID = uuid.New()

// auditEnv is the profile, project and scheme graph owned by a test.
type auditEnv struct {
	admin, user, user1 factory.Profile

	project      factory.Project
	member       factory.ProjectMember
	scheme       factory.Scheme
	schemeMember factory.SchemeMember
}

func setupAudit(t *testing.T) (*test.GRPCHandler, *auditEnv, func(t *testing.T)) {
	t.Helper()

	setup, teardownTestCase := test.GRPC(t)
	builder := factory.New(t, setup.Postgres.Conn().Conn)

	env := &auditEnv{}
	env.admin = builder.Profile(factory.ProfileParams{Alias: "admin", Role: 3})
	env.user = builder.Profile(factory.ProfileParams{Alias: "user", Role: 1})
	env.user1 = builder.Profile(factory.ProfileParams{Alias: "user1", Role: 1})

	env.project = builder.Project(factory.ProjectParams{OwnerID: env.admin.ID})
	env.member = builder.ProjectMember(factory.ProjectMemberParams{
		ProjectID: env.project.ID,
		ProfileID: env.user.ID,
	})
	env.scheme = builder.Scheme(factory.SchemeParams{
		ProjectID:  env.project.ID,
		SchemeType: 103,
	})
	env.schemeMember = builder.SchemeMember(factory.SchemeMemberParams{
		SchemeID:        env.scheme.ID,
		ProjectMemberID: env.member.ID,
	})

	return setup, env, teardownTestCase
}

func addAudit(t *testing.T, client auditrpc.AuditHandlersClient, env *auditEnv, profileID, session string) *auditmessage.AddAudit_Response {
	t.Helper()

	created, err := client.AddAudit(context.Background(), &auditmessage.AddAudit_Request{
		ServerId:  env.scheme.ID,
		AccountId: env.schemeMember.ID,
		ProfileId: profileID,
		Version:   2,
		Session:   session,
		ClientIp:  "8.8.8.8",
	})
	if err != nil {
		t.Fatalf("prepare audit: %v", err)
	}

	return created
}

func Test_AddAudit(t *testing.T) {
	setup, _, teardownTestCase := setupAudit(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := auditrpc.NewAuditHandlersClient(setup)
		return a.AddAudit(ctx, req.(*auditmessage.AddAudit_Request))
	}

	testTable := []test.GRPCTable{
		{ // request without parameters
			Name:    "test0_01",
			Request: &auditmessage.AddAudit_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"account_id": "value is required",
					"profile_id": "value is required",
					"server_id":  "value is required",
				},
			},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			test.RunCaseGRPCTests(t, handler, tt)
		})
	}
}

func Test_AddAudit_writesRow(t *testing.T) {
	setup, env, teardownTestCase := setupAudit(t)
	defer teardownTestCase(t)

	client := auditrpc.NewAuditHandlersClient(setup)

	t.Run("unknown_member_is_rejected", func(t *testing.T) {
		_, err := client.AddAudit(context.Background(), &auditmessage.AddAudit_Request{
			ServerId:  unknownID,
			AccountId: unknownID,
			ProfileId: env.admin.ID,
			Version:   2,
			Session:   "session-1",
			ClientIp:  "8.8.8.8",
		})
		if err == nil {
			t.Fatal("expected an error for a member that does not exist")
		}
		if code := status.Code(err); code != codes.NotFound {
			t.Fatalf("code = %v, want NotFound", code)
		}
	})

	t.Run("member_creates_audit", func(t *testing.T) {
		created := addAudit(t, client, env, env.user.ID, "session-1")
		if created.GetAuditId() == "" {
			t.Fatal("audit_id is empty")
		}

		resp, err := client.Audit(context.Background(), &auditmessage.Audit_Request{
			AuditId:   created.GetAuditId(),
			ProfileId: env.user.ID,
		})
		if err != nil {
			t.Fatalf("audit by id: %v", err)
		}
		if resp.GetCreatedAt() == nil {
			t.Fatal("created_at is empty")
		}
	})

	t.Run("owner_creates_audit", func(t *testing.T) {
		created := addAudit(t, client, env, env.admin.ID, "session-owner")
		if created.GetAuditId() == "" {
			t.Fatal("audit_id is empty")
		}
	})

	t.Run("non_member_is_denied", func(t *testing.T) {
		_, err := client.AddAudit(context.Background(), &auditmessage.AddAudit_Request{
			ServerId:  env.scheme.ID,
			AccountId: env.schemeMember.ID,
			ProfileId: env.user1.ID,
			Version:   2,
			Session:   "session-denied",
			ClientIp:  "8.8.8.8",
		})
		if code := status.Code(err); code != codes.PermissionDenied {
			t.Fatalf("code = %v, want PermissionDenied", code)
		}
	})
}

func Test_UpdateAudit(t *testing.T) {
	setup, env, teardownTestCase := setupAudit(t)
	defer teardownTestCase(t)

	client := auditrpc.NewAuditHandlersClient(setup)
	created := addAudit(t, client, env, env.user.ID, "session-update")

	t.Run("unknown_audit_returns_not_found", func(t *testing.T) {
		_, err := client.UpdateAudit(context.Background(), &auditmessage.UpdateAudit_Request{
			AuditId:   unknownID,
			ProfileId: env.admin.ID,
			Session:   "session-update",
			Duration:  "1.5",
		})
		if err == nil {
			t.Fatal("expected an error for an unknown audit")
		}
		if code := status.Code(err); code != codes.NotFound {
			t.Fatalf("code = %v, want NotFound", code)
		}
	})

	t.Run("closes_the_audit", func(t *testing.T) {
		_, err := client.UpdateAudit(context.Background(), &auditmessage.UpdateAudit_Request{
			AuditId:   created.GetAuditId(),
			ProfileId: env.user.ID,
			Session:   "session-update",
			Duration:  "12.500000",
			TimeEnd:   timestamppb.Now(),
		})
		if err != nil {
			t.Fatalf("close audit: %v", err)
		}
	})

	t.Run("keeps_previous_values", func(t *testing.T) {
		// Only the terminal size is sent; duration and time_end must survive.
		if _, err := client.UpdateAudit(context.Background(), &auditmessage.UpdateAudit_Request{
			AuditId:   created.GetAuditId(),
			ProfileId: env.admin.ID,
			Session:   "session-update",
			Width:     120,
			Height:    40,
			EnvTerm:   "xterm-256color",
		}); err != nil {
			t.Fatalf("update terminal size: %v", err)
		}

		resp, err := client.Audit(context.Background(), &auditmessage.Audit_Request{
			AuditId:   created.GetAuditId(),
			ProfileId: env.admin.ID,
		})
		if err != nil {
			t.Fatalf("audit by id: %v", err)
		}
		if resp.GetUpdatedAt() == nil {
			t.Fatal("time_end was overwritten by a request that did not set it")
		}
	})
}

func Test_AuditRecords(t *testing.T) {
	setup, env, teardownTestCase := setupAudit(t)
	defer teardownTestCase(t)

	client := auditrpc.NewAuditHandlersClient(setup)
	created := addAudit(t, client, env, env.user.ID, "session-records")
	auditID := created.GetAuditId()

	if _, err := client.AddRecord(context.Background(), &auditmessage.AddRecord_Request{
		AuditId:   auditID,
		ProfileId: env.user.ID,
		Session:   "session-records",
		Records: []*auditmessage.Record{
			{Duration: "0.100000", Screen: "hello", Type: "o"},
			{Duration: "0.200000", Screen: "world", Type: "o"},
		},
	}); err != nil {
		t.Fatalf("add records: %v", err)
	}

	t.Run("records_are_stored", func(t *testing.T) {
		resp, err := client.ListRecords(context.Background(), &auditmessage.ListRecords_Request{
			AuditId:   auditID,
			ProfileId: env.user.ID,
		})
		if err != nil {
			t.Fatalf("list records: %v", err)
		}
		if got := len(resp.GetRecords()); got != 2 {
			t.Fatalf("records count = %d, want 2", got)
		}
		if got := resp.GetRecords()[0].GetScreen(); got != "hello" {
			t.Fatalf("first record screen = %q, want hello", got)
		}
	})

	t.Run("audit_is_listed_for_the_scheme", func(t *testing.T) {
		resp, err := client.ListAudits(context.Background(), &auditmessage.ListAudits_Request{
			ServerId:  env.scheme.ID,
			ProfileId: env.user.ID,
		})
		if err != nil {
			t.Fatalf("list audits: %v", err)
		}
		if resp.GetTotal() < 1 {
			t.Fatalf("audits total = %d, want at least 1", resp.GetTotal())
		}
	})

	t.Run("records_are_removed_with_the_audit", func(t *testing.T) {
		if _, err := client.DeleteAudit(context.Background(), &auditmessage.DeleteAudit_Request{
			AuditId:   auditID,
			ProfileId: env.user.ID,
			Session:   "session-records",
		}); err != nil {
			t.Fatalf("delete audit: %v", err)
		}

		resp, err := client.ListRecords(context.Background(), &auditmessage.ListRecords_Request{
			AuditId:   auditID,
			ProfileId: env.user.ID,
		})
		if err != nil {
			t.Fatalf("list records after delete: %v", err)
		}
		if got := len(resp.GetRecords()); got != 0 {
			t.Fatalf("records count after delete = %d, want 0", got)
		}
	})
}

func Test_AuditAuthorization(t *testing.T) {
	setup, env, teardownTestCase := setupAudit(t)
	defer teardownTestCase(t)

	client := auditrpc.NewAuditHandlersClient(setup)
	created := addAudit(t, client, env, env.user.ID, "session-auth")
	auditID := created.GetAuditId()

	t.Run("member_can_read", func(t *testing.T) {
		if _, err := client.Audit(context.Background(), &auditmessage.Audit_Request{
			AuditId:   auditID,
			ProfileId: env.user.ID,
		}); err != nil {
			t.Fatalf("member read: %v", err)
		}
	})

	t.Run("owner_can_read", func(t *testing.T) {
		if _, err := client.Audit(context.Background(), &auditmessage.Audit_Request{
			AuditId:   auditID,
			ProfileId: env.admin.ID,
		}); err != nil {
			t.Fatalf("owner read: %v", err)
		}
	})

	t.Run("non_member_read_is_not_found", func(t *testing.T) {
		_, err := client.Audit(context.Background(), &auditmessage.Audit_Request{
			AuditId:   auditID,
			ProfileId: env.user1.ID,
		})
		if code := status.Code(err); code != codes.NotFound {
			t.Fatalf("code = %v, want NotFound", code)
		}
	})

	t.Run("non_member_update_is_denied", func(t *testing.T) {
		_, err := client.UpdateAudit(context.Background(), &auditmessage.UpdateAudit_Request{
			AuditId:   auditID,
			ProfileId: env.user1.ID,
			Session:   "session-auth",
			Duration:  "1.5",
		})
		if code := status.Code(err); code != codes.PermissionDenied {
			t.Fatalf("code = %v, want PermissionDenied", code)
		}
	})

	t.Run("non_member_add_record_is_denied", func(t *testing.T) {
		_, err := client.AddRecord(context.Background(), &auditmessage.AddRecord_Request{
			AuditId:   auditID,
			ProfileId: env.user1.ID,
			Session:   "session-auth",
			Records: []*auditmessage.Record{
				{Duration: "0.100000", Screen: "denied", Type: "o"},
			},
		})
		if code := status.Code(err); code != codes.PermissionDenied {
			t.Fatalf("code = %v, want PermissionDenied", code)
		}
	})

	t.Run("non_member_sees_no_records", func(t *testing.T) {
		resp, err := client.ListRecords(context.Background(), &auditmessage.ListRecords_Request{
			AuditId:   auditID,
			ProfileId: env.user1.ID,
		})
		if err != nil {
			t.Fatalf("list records: %v", err)
		}
		if got := len(resp.GetRecords()); got != 0 {
			t.Fatalf("records count = %d, want 0", got)
		}
	})

	t.Run("list_audits_is_filtered_by_membership", func(t *testing.T) {
		member, err := client.ListAudits(context.Background(), &auditmessage.ListAudits_Request{
			ServerId:  env.scheme.ID,
			ProfileId: env.user.ID,
		})
		if err != nil {
			t.Fatalf("member list audits: %v", err)
		}
		if member.GetTotal() < 1 {
			t.Fatalf("member audits total = %d, want at least 1", member.GetTotal())
		}

		other, err := client.ListAudits(context.Background(), &auditmessage.ListAudits_Request{
			ServerId:  env.scheme.ID,
			ProfileId: env.user1.ID,
		})
		if err != nil {
			t.Fatalf("non-member list audits: %v", err)
		}
		if other.GetTotal() != 0 {
			t.Fatalf("non-member audits total = %d, want 0", other.GetTotal())
		}
	})

	t.Run("wrong_session_update_is_denied", func(t *testing.T) {
		_, err := client.UpdateAudit(context.Background(), &auditmessage.UpdateAudit_Request{
			AuditId:   auditID,
			ProfileId: env.user.ID,
			Session:   "another-session",
			Duration:  "1.5",
		})
		if code := status.Code(err); code != codes.PermissionDenied {
			t.Fatalf("code = %v, want PermissionDenied", code)
		}
	})

	t.Run("non_member_delete_is_denied", func(t *testing.T) {
		_, err := client.DeleteAudit(context.Background(), &auditmessage.DeleteAudit_Request{
			AuditId:   auditID,
			ProfileId: env.user1.ID,
			Session:   "session-auth",
		})
		if code := status.Code(err); code != codes.PermissionDenied {
			t.Fatalf("code = %v, want PermissionDenied", code)
		}
	})

	t.Run("owner_can_delete", func(t *testing.T) {
		if _, err := client.DeleteAudit(context.Background(), &auditmessage.DeleteAudit_Request{
			AuditId:   auditID,
			ProfileId: env.admin.ID,
			Session:   "session-auth",
		}); err != nil {
			t.Fatalf("owner delete: %v", err)
		}
	})
}
