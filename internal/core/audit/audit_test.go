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
)

// constSchemeMemberID is a "scheme_member" row from the fixtures. The audit
// table references that table through "member_id".
const constSchemeMemberID = "8ca0ad93-4338-44cb-93dd-3f57272d0ffa"

func Test_AddAudit(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
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
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	client := auditrpc.NewAuditHandlersClient(setup)

	t.Run("unknown_member_is_rejected", func(t *testing.T) {
		_, err := client.AddAudit(context.Background(), &auditmessage.AddAudit_Request{
			ServerId:  test.ConstFakeID,
			AccountId: test.ConstFakeID,
			Version:   2,
			Session:   "session-1",
			ClientIp:  "8.8.8.8",
		})
		if err == nil {
			t.Fatal("expected an error for a member that does not exist")
		}
	})

	created, err := client.AddAudit(context.Background(), &auditmessage.AddAudit_Request{
		ServerId:  test.ConstAdminSchemeSSH1ID,
		AccountId: constSchemeMemberID,
		Version:   2,
		Session:   "session-1",
		ClientIp:  "8.8.8.8",
	})
	if err != nil {
		t.Fatalf("add audit: %v", err)
	}
	if created.GetAuditId() == "" {
		t.Fatal("audit_id is empty")
	}

	t.Run("audit_is_readable", func(t *testing.T) {
		resp, err := client.Audit(context.Background(), &auditmessage.Audit_Request{
			AuditId: created.GetAuditId(),
		})
		if err != nil {
			t.Fatalf("audit by id: %v", err)
		}
		if resp.GetCreatedAt() == nil {
			t.Fatal("created_at is empty")
		}
	})
}

func Test_UpdateAudit(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	client := auditrpc.NewAuditHandlersClient(setup)

	created, err := client.AddAudit(context.Background(), &auditmessage.AddAudit_Request{
		ServerId:  test.ConstAdminSchemeSSH1ID,
		AccountId: constSchemeMemberID,
		Version:   2,
		Session:   "session-update",
		ClientIp:  "8.8.8.8",
	})
	if err != nil {
		t.Fatalf("prepare audit: %v", err)
	}

	t.Run("unknown_audit_returns_not_found", func(t *testing.T) {
		_, err := client.UpdateAudit(context.Background(), &auditmessage.UpdateAudit_Request{
			AuditId:  test.ConstFakeID,
			Duration: "1.5",
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
			AuditId:  created.GetAuditId(),
			Duration: "12.500000",
			TimeEnd:  timestamppb.Now(),
		})
		if err != nil {
			t.Fatalf("close audit: %v", err)
		}
	})

	t.Run("keeps_previous_values", func(t *testing.T) {
		// Only the terminal size is sent; duration and time_end must survive.
		if _, err := client.UpdateAudit(context.Background(), &auditmessage.UpdateAudit_Request{
			AuditId: created.GetAuditId(),
			Width:   120,
			Height:  40,
			EnvTerm: "xterm-256color",
		}); err != nil {
			t.Fatalf("update terminal size: %v", err)
		}

		resp, err := client.Audit(context.Background(), &auditmessage.Audit_Request{
			AuditId: created.GetAuditId(),
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
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	client := auditrpc.NewAuditHandlersClient(setup)

	created, err := client.AddAudit(context.Background(), &auditmessage.AddAudit_Request{
		ServerId:  test.ConstAdminSchemeSSH1ID,
		AccountId: constSchemeMemberID,
		Version:   2,
		Session:   "session-records",
		ClientIp:  "8.8.8.8",
	})
	if err != nil {
		t.Fatalf("prepare audit: %v", err)
	}
	auditID := created.GetAuditId()

	if _, err := client.AddRecord(context.Background(), &auditmessage.AddRecord_Request{
		AuditId: auditID,
		Records: []*auditmessage.Record{
			{Duration: "0.100000", Screen: "hello", Type: "o"},
			{Duration: "0.200000", Screen: "world", Type: "o"},
		},
	}); err != nil {
		t.Fatalf("add records: %v", err)
	}

	t.Run("records_are_stored", func(t *testing.T) {
		resp, err := client.ListRecords(context.Background(), &auditmessage.ListRecords_Request{
			AuditId: auditID,
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
			ServerId: test.ConstAdminSchemeSSH1ID,
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
			AuditId: auditID,
		}); err != nil {
			t.Fatalf("delete audit: %v", err)
		}

		resp, err := client.ListRecords(context.Background(), &auditmessage.ListRecords_Request{
			AuditId: auditID,
		})
		if err != nil {
			t.Fatalf("list records after delete: %v", err)
		}
		if got := len(resp.GetRecords()); got != 0 {
			t.Fatalf("records count after delete = %d, want 0", got)
		}
	})
}
