package grpc

import (
	"context"
	"time"

	auditpb "github.com/werbot/werbot/api/proto/audit"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
)

type audit struct {
	auditpb.UnimplementedAuditHandlersServer
}

// TODO ListAudits is displays the list all audits for server_id
func (a *audit) ListAudits(ctx context.Context, in *auditpb.ListAudits_Request) (*auditpb.ListAudits_Response, error) {
	response := new(auditpb.ListAudits_Response)
	return response, nil
}

// TODO Audit is displays audit information on audit_id
func (a *audit) Audit(ctx context.Context, in *auditpb.Audit_Request) (*auditpb.Audit_Response, error) {
	response := new(auditpb.Audit_Response)
	return response, nil
}

// AddAudit is adds a new audit for server_id
func (a *audit) AddAudit(ctx context.Context, in *auditpb.AddAudit_Request) (*auditpb.AddAudit_Response, error) {
	if in.GetAccountId() == "" && in.GetVersion() == 0 && in.GetSession() == "" && in.GetClientIp() == "" {
		return nil, errIncorrectParameters
	}

	response := new(auditpb.AddAudit_Response)

	err := service.db.Conn.QueryRow(`INSERT INTO "audit" ("account_id", "time_start", "version", "width", "height", "duration", "command", "title", "env_term", "env_shell", "session", "client_ip")
		VALUES ($1, $2, $3, 0, 0, '0', '', $4, '', '/bin/sh', $4, $5)
		RETURNING "id"`,
		in.GetAccountId(),
		time.Now(),
		in.GetVersion(),
		in.GetSession(),
		in.GetClientIp(),
	).Scan(&response.AuditId)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}

	return response, nil
}

// UpdateAudit is update audit for server_id
func (a *audit) UpdateAudit(ctx context.Context, in *auditpb.UpdateAudit_Request) (*auditpb.UpdateAudit_Response, error) {
	response := new(auditpb.UpdateAudit_Response)

	data, err := service.db.Conn.Exec(`UPDATE "audit" SET "duration" = $1, "time_end" = $2 WHERE "id" = $3`,
		in.GetAuditId(),
		in.GetDuration(),
		in.GetTimeEnd().AsTime(),
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return response, nil
}

// TODO DeleteAudit is delete audit for server_id
func (a *audit) DeleteAudit(ctx context.Context, in *auditpb.DeleteAudit_Request) (*auditpb.DeleteAudit_Response, error) {
	response := new(auditpb.DeleteAudit_Response)
	return response, nil
}

// TODO ListRecords is display of all records for audit_id
func (a *audit) ListRecords(ctx context.Context, in *auditpb.ListRecords_Request) (*auditpb.ListRecords_Response, error) {
	response := new(auditpb.ListRecords_Response)
	return response, nil
}

// AddRecord is adds a new record for audit_id
// https://git.piplos.by/werbot/old-werbot/-/blob/master/wserver/audit.go
func (a *audit) AddRecord(ctx context.Context, in *auditpb.AddRecord_Request) (*auditpb.AddRecord_Response, error) {
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

	data, err := service.db.Conn.Exec(query)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return response, nil
}
