package logging

import (
	"context"
	"database/sql"

	loggingpb "github.com/werbot/werbot/api/proto/logging"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
)

// ListRecords is ...
func (h *Handler) ListRecords(ctx context.Context, in *loggingpb.ListRecords_Request) (*loggingpb.ListRecords_Response, error) {
	response := new(loggingpb.ListRecords_Response)
	return response, nil
}

// Record is ...
func (h *Handler) Record(ctx context.Context, in *loggingpb.Record_Request) (*loggingpb.Record_Response, error) {
	response := new(loggingpb.Record_Response)
	return response, nil
}

// AddLogRecord is ...
func (h *Handler) AddLogRecord(ctx context.Context, in *loggingpb.AddRecord_Request) (*loggingpb.AddRecord_Response, error) {
	var sqlQuery string
	var data sql.Result
	var err error
	response := new(loggingpb.AddRecord_Response)

	switch in.GetLogger() {
	case loggingpb.Logger_profile:
		sqlQuery, err = sanitize.SQL(`INSERT INTO "logs_profile" ("profile_id", "entity_id", "entity_name", "editor_name", "editor_role", "user_agent", "ip", "event", "data")
      VALUES ($1, '', '', '', '', '' '', $2, '')`,
			in.GetId(),
			in.GetEvent().String(),
		)
		if err != nil {
			log.FromGRPC(err).Send()
			return nil, errBadRequest
		}

	case loggingpb.Logger_project:
		sqlQuery, err = sanitize.SQL(`INSERT INTO "logs_project" ("project_id", "entity_id", "entity_name", "editor_name", "editor_role", "user_agent", "ip", "event", "data")
      VALUES ($1, '', '', '', '', '' '', $2, '')`,
			in.GetId(),
			in.GetEvent().String(),
		)
		if err != nil {
			log.FromGRPC(err).Send()
			return nil, errBadRequest
		}
	}

	if data, err = h.DB.Conn.Exec(sqlQuery); err != nil {
		log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errFailedToAdd
	}

	return response, nil
}
