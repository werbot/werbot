package audit

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	auditmessage "github.com/werbot/werbot/internal/core/audit/proto/message"
	"github.com/werbot/werbot/internal/trace"
)

// ListAudits is displays the list all audits for scheme_id
func (h *Handler) ListAudits(ctx context.Context, in *auditmessage.ListAudits_Request) (*auditmessage.ListAudits_Response, error) {
	response := &auditmessage.ListAudits_Response{}
	return response, nil
}

// Audit is displays audit information on audit_id
func (h *Handler) Audit(ctx context.Context, in *auditmessage.Audit_Request) (*auditmessage.Audit_Response, error) {
	response := &auditmessage.Audit_Response{}
	return response, nil
}

// AddAudit is adds a new audit for scheme_id
func (h *Handler) AddAudit(ctx context.Context, in *auditmessage.AddAudit_Request) (*auditmessage.AddAudit_Response, error) {
	if in.GetAccountId() == "" && in.GetVersion() == 0 && in.GetSession() == "" && in.GetClientIp() == "" {
		return nil, status.Error(codes.InvalidArgument, trace.MsgInvalidArgument)
	}

	response := &auditmessage.AddAudit_Response{}

	err := h.DB.Conn.QueryRowContext(ctx, `
    INSERT INTO
      "audit" (
        "account_id",
        "time_start",
        "version",
        "width",
        "height",
        "duration",
        "command",
        "title",
        "env_term",
        "env_shell",
        "session",
        "client_ip"
      )
    VALUES ($1, NOW(), $3, 0, 0, 0, '', '', '', '/bin/sh', $4)
    RETURNING "id"
  `,
		in.GetAccountId(),
		in.GetVersion(),
		in.GetSession(),
		in.GetClientIp(),
	).Scan(&response.AuditId)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToAdd)
	}

	return response, nil
}

// UpdateAudit is update audit for scheme_id
func (h *Handler) UpdateAudit(ctx context.Context, in *auditmessage.UpdateAudit_Request) (*auditmessage.UpdateAudit_Response, error) {
	response := &auditmessage.UpdateAudit_Response{}

	res, err := h.DB.Conn.ExecContext(ctx, `
    UPDATE "audit"
    SET
      "duration" = $1,
      "time_end" = $2
    WHERE "id" = $3
  `,
		in.GetDuration(),
		in.GetTimeEnd().AsTime(),
		in.GetAuditId(),
	)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgAuditNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return response, nil
}

// DeleteAudit is delete audit for scheme_id
func (h *Handler) DeleteAudit(ctx context.Context, in *auditmessage.DeleteAudit_Request) (*auditmessage.DeleteAudit_Response, error) {
	response := &auditmessage.DeleteAudit_Response{}
	return response, nil
}

// ListRecords is display of all records for audit_id
func (h *Handler) ListRecords(ctx context.Context, in *auditmessage.ListRecords_Request) (*auditmessage.ListRecords_Response, error) {
	response := &auditmessage.ListRecords_Response{}
	return response, nil
}

// AddRecord is adds a new record for audit_id
// https://git.piplos.by/werbot/old-werbot/-/blob/master/wserver/audit.go
func (h *Handler) AddRecord(ctx context.Context, in *auditmessage.AddRecord_Request) (*auditmessage.AddRecord_Response, error) {
	response := &auditmessage.AddRecord_Response{}

	tx, err := h.DB.Conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgTransactionCreateError)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
    INSERT INTO "audit_record" ("audit_id", "duration", "screen", "type")
    VALUES ($1, $2, $3, $4)
  `)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToAdd)
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
			return nil, trace.Error(err, log, trace.MsgFailedToAdd)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, trace.Error(err, log, trace.MsgTransactionCommitError)
	}

	return response, nil
}
