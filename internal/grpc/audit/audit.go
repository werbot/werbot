package audit

import (
	"context"

	auditpb "github.com/werbot/werbot/api/proto/audit"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
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
		return nil, errIncorrectParameters
	}

	response := new(auditpb.AddAudit_Response)

	err := h.DB.Conn.QueryRow(`INSERT INTO "audit" ("account_id", "time_start", "version", "width", "height", "duration", "command", "title", "env_term", "env_shell", "session", "client_ip")
		VALUES ($1, NOW(), $3, 0, 0, 0, '', '', '', '/bin/sh', $4)
		RETURNING "id"`,
		in.GetAccountId(),
		in.GetVersion(),
		in.GetSession(),
		in.GetClientIp(),
	).Scan(&response.AuditId)
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}

	return response, nil
}

// UpdateAudit is update audit for server_id
func (h *Handler) UpdateAudit(ctx context.Context, in *auditpb.UpdateAudit_Request) (*auditpb.UpdateAudit_Response, error) {
	response := new(auditpb.UpdateAudit_Response)

	data, err := h.DB.Conn.Exec(`UPDATE "audit" SET "duration" = $1 WHERE "id" = $3`,
		in.GetAuditId(),
		in.GetDuration(),
		in.GetTimeEnd().AsTime(),
	)
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
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

	query := `INSERT INTO "audit_record" ("audit_id", "duration", "screen", "type") VALUES `
	for _, param := range in.GetRecords() {
		sanitizeSQL, _ := sanitize.SQL(`($1, $2, $3, $4),`,
			in.GetAuditId(),
			param.Duration,
			param.Screen,
			param.Type,
		)
		query += sanitizeSQL
	}
	query = query[:len(query)-1]

	data, err := h.DB.Conn.Exec(query)
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return response, nil
}
