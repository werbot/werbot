package grpc

import (
	"context"
	"strconv"
	"time"

	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
	"github.com/werbot/werbot/internal/utils/convert"

	pb_audit "github.com/werbot/werbot/api/proto/audit"
)

type audit struct {
	pb_audit.UnimplementedAuditHandlersServer
}

// CreateAudit is ...
func (s *audit) CreateAudit(ctx context.Context, in *pb_audit.CreateAudit_Request) (*pb_audit.CreateAudit_Response, error) {
	if in.GetAccountId() == "" && in.GetVersion() == 0 && in.GetSession() == "" && in.GetClientIp() == "" {
		return nil, errIncorrectParameters
	}

	var auditID string
	err := service.db.Conn.QueryRow(`INSERT
		INTO "audit" (
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
			"client_ip")
		VALUES
			($1, $2, $3, 0, 0, '0', '', $4, '', '/bin/sh', $4, $5)
		RETURNING "id"`,
		in.GetAccountId(),
		time.Now(),
		in.GetVersion(),
		in.GetSession(),
		in.GetClientIp(),
	).Scan(&auditID)
	if err != nil {
		return nil, errFailedToAdd
	}

	return &pb_audit.CreateAudit_Response{
		AuditId: auditID,
	}, nil
}

// UpdateAudit is ...
func (s *audit) UpdateAudit(ctx context.Context, in *pb_audit.UpdateAudit_Request) (*pb_audit.UpdateAudit_Response, error) {
	var values []any
	query := `UPDATE "audit" SET `
	j := 0
	m := convert.StructToMap(in.GetParams())
	for i := range m {
		if v := m[i]; v != "" && v != 0 {
			j++
			query = query + i + "=$" + strconv.Itoa(j) + ","
			values = append(values, v)
		}
	}
	sanitizeSQL, _ := sanitize.SQL(` WHERE "id" = $1`, in.GetAuditId())
	query = query[:len(query)-1] + sanitizeSQL
	data, err := service.db.Conn.Exec(query, values...)
	if err != nil {
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return &pb_audit.UpdateAudit_Response{}, nil
}

// CreateRecord is ...
// https://git.piplos.by/werbot/old-werbot/-/blob/master/wserver/audit.go
func (s *audit) CreateRecord(ctx context.Context, in *pb_audit.CreateRecord_Request) (*pb_audit.CreateRecord_Response, error) {
	query := `INSERT INTO "audit_record" ("audit_id", "duration", "screen", "type") VALUES `
	for _, param := range in.GetRecords() {
		sanitizeSQL, _ := sanitize.SQL(`($1, $2, $3, $4),`, in.AuditId, param.Duration, param.Screen, param.Type)
		query += sanitizeSQL
	}
	query = query[:len(query)-1]
	_, err := service.db.Conn.Exec(query)
	if err != nil {
		return nil, errFailedToAdd
	}

	return &pb_audit.CreateRecord_Response{}, nil
}
