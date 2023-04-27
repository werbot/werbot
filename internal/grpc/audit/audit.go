package audit

import (
	"context"

	auditpb "github.com/werbot/werbot/internal/grpc/audit/proto"
	"github.com/werbot/werbot/internal/trace"
	"google.golang.org/grpc/codes"
)

// ListAudits is displays the list all audits for server_id
func (h *Handler) ListAudits(ctx context.Context, in *auditpb.ListAudits_Request) (*auditpb.ListAudits_Response, error) {
	response := new(auditpb.ListAudits_Response)
	return response, nil
}

// Audit is displays audit information on audit_id
func (h *Handler) Audit(ctx context.Context, in *auditpb.Audit_Request) (*auditpb.Audit_Response, error) {
	response := new(auditpb.Audit_Response)
	return response, nil
}

// AddAudit is adds a new audit for server_id
func (h *Handler) AddAudit(ctx context.Context, in *auditpb.AddAudit_Request) (*auditpb.AddAudit_Response, error) {
	if in.GetAccountId() == "" && in.GetVersion() == 0 && in.GetSession() == "" && in.GetClientIp() == "" {
		return nil, trace.Error(codes.InvalidArgument)
	}

	response := new(auditpb.AddAudit_Response)

	err := h.DB.Conn.QueryRowContext(ctx, `INSERT INTO "audit" ("account_id", "time_start", "version", "width", "height", "duration", "command", "title", "env_term", "env_shell", "session", "client_ip")
		VALUES ($1, NOW(), $3, 0, 0, 0, '', '', '', '/bin/sh', $4)
		RETURNING "id"`,
		in.GetAccountId(),
		in.GetVersion(),
		in.GetSession(),
		in.GetClientIp(),
	).Scan(&response.AuditId)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log, trace.MsgFailedToAdd)
	}

	return response, nil
}

// UpdateAudit is update audit for server_id
func (h *Handler) UpdateAudit(ctx context.Context, in *auditpb.UpdateAudit_Request) (*auditpb.UpdateAudit_Response, error) {
	response := new(auditpb.UpdateAudit_Response)

	_, err := h.DB.Conn.ExecContext(ctx, `UPDATE "audit" SET "duration" = $1, "time_end" = $2 WHERE "id" = $3`,
		in.GetDuration(),
		in.GetTimeEnd().AsTime(),
		in.GetAuditId(),
	)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log, trace.MsgFailedToUpdate)
	}

	return response, nil
}

// DeleteAudit is delete audit for server_id
func (h *Handler) DeleteAudit(ctx context.Context, in *auditpb.DeleteAudit_Request) (*auditpb.DeleteAudit_Response, error) {
	response := new(auditpb.DeleteAudit_Response)
	return response, nil
}

// ListRecords is display of all records for audit_id
func (h *Handler) ListRecords(ctx context.Context, in *auditpb.ListRecords_Request) (*auditpb.ListRecords_Response, error) {
	response := new(auditpb.ListRecords_Response)
	return response, nil
}

// AddRecord is adds a new record for audit_id
// https://git.piplos.by/werbot/old-werbot/-/blob/master/wserver/audit.go
func (h *Handler) AddRecord(ctx context.Context, in *auditpb.AddRecord_Request) (*auditpb.AddRecord_Response, error) {
	response := new(auditpb.AddRecord_Response)

	tx, err := h.DB.Conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log, trace.MsgTransactionCreateError)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO "audit_record" ("audit_id", "duration", "screen", "type") VALUES ($1, $2, $3, $4)`)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log, trace.MsgFailedToAdd)
	}
	defer stmt.Close()

	for _, param := range in.GetRecords() {
		_, err = stmt.ExecContext(ctx,
			in.GetAuditId(),
			param.Duration,
			param.Screen,
			param.Type,
		)
		if err != nil {
			return nil, trace.ErrorAborted(err, h.Log, trace.MsgFailedToAdd)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, trace.ErrorAborted(err, h.Log, trace.MsgTransactionCommitError)
	}

	return response, nil
}
