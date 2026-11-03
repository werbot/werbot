package audit

import (
	"context"
	"database/sql"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	auditmessage "github.com/werbot/werbot/internal/core/audit/proto/message"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/utils/protoutils"
)

// ListAudits is displays the list all audits for scheme_id
func (h *Handler) ListAudits(ctx context.Context, in *auditmessage.ListAudits_Request) (*auditmessage.ListAudits_Response, error) {
	response := &auditmessage.ListAudits_Response{}

	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT COUNT(*)
    FROM
      "audit"
      INNER JOIN "scheme_member" ON "audit"."member_id" = "scheme_member"."id"
    WHERE "scheme_member"."scheme_id" = $1
  `, in.GetServerId()).Scan(&response.Total)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, trace.Error(err, log, nil)
	}

	return response, nil
}

// Audit is displays audit information on audit_id
func (h *Handler) Audit(ctx context.Context, in *auditmessage.Audit_Request) (*auditmessage.Audit_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	response := &auditmessage.Audit_Response{}

	var timeStart, timeEnd sql.NullTime
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT "time_start", "time_end"
    FROM "audit"
    WHERE "id" = $1
  `, in.GetAuditId()).Scan(&timeStart, &timeEnd)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errGRPC := status.Error(codes.NotFound, trace.MsgAuditNotFound)
			return nil, trace.Error(errGRPC, log, nil)
		}
		return nil, trace.Error(err, log, nil)
	}

	if timeStart.Valid {
		response.CreatedAt = timestamppb.New(timeStart.Time)
	}
	if timeEnd.Valid {
		response.UpdatedAt = timestamppb.New(timeEnd.Time)
	}

	return response, nil
}

// AddAudit is adds a new audit for scheme_id
func (h *Handler) AddAudit(ctx context.Context, in *auditmessage.AddAudit_Request) (*auditmessage.AddAudit_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	response := &auditmessage.AddAudit_Response{}

	err := h.DB.Conn.QueryRowContext(ctx, `
    INSERT INTO
      "audit" (
        "member_id",
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
    VALUES ($1, NOW(), $2, 0, 0, 0, '', '', '', '/bin/sh', $3, $4)
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
      "width" = COALESCE(NULLIF($1, 0), "width"),
      "height" = COALESCE(NULLIF($2, 0), "height"),
      "env_term" = COALESCE(NULLIF($3, ''), "env_term"),
      "duration" = COALESCE(NULLIF($4, '')::float8, "duration"),
      "time_end" = COALESCE($5, "time_end")
    WHERE "id" = $6
  `,
		in.GetWidth(),
		in.GetHeight(),
		in.GetEnvTerm(),
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
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	res, err := h.DB.Conn.ExecContext(ctx, `
    DELETE FROM "audit"
    WHERE "id" = $1
  `, in.GetAuditId())
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToDelete)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgAuditNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return &auditmessage.DeleteAudit_Response{}, nil
}

// ListRecords is display of all records for audit_id
func (h *Handler) ListRecords(ctx context.Context, in *auditmessage.ListRecords_Request) (*auditmessage.ListRecords_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	response := &auditmessage.ListRecords_Response{}

	rows, err := h.DB.Conn.QueryContext(ctx, `
    SELECT "id", "duration", "screen", "type"
    FROM "audit_record"
    WHERE "audit_id" = $1
    ORDER BY "duration"
  `, in.GetAuditId())
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	defer rows.Close()

	for rows.Next() {
		record := &auditmessage.Record{}
		if err := rows.Scan(&record.Id, &record.Duration, &record.Screen, &record.Type); err != nil {
			return nil, trace.Error(err, log, nil)
		}
		response.Records = append(response.Records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, trace.Error(err, log, nil)
	}

	return response, nil
}

// AddRecord is adds a new record for audit_id
func (h *Handler) AddRecord(ctx context.Context, in *auditmessage.AddRecord_Request) (*auditmessage.AddRecord_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

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
		if _, err := stmt.ExecContext(ctx,
			in.GetAuditId(),
			param.GetDuration(),
			param.GetScreen(),
			param.GetType(),
		); err != nil {
			return nil, trace.Error(err, log, trace.MsgFailedToAdd)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, trace.Error(err, log, trace.MsgTransactionCommitError)
	}

	return &auditmessage.AddRecord_Response{}, nil
}
