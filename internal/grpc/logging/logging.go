package logging

import (
	"context"

	loggingpb "github.com/werbot/werbot/internal/grpc/logging/proto"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
	"github.com/werbot/werbot/internal/trace"
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
			return nil, trace.ErrorAborted(err, h.Log)
		}

	case loggingpb.Logger_project:
		sqlQuery, err = sanitize.SQL(`INSERT INTO "logs_project" ("project_id", "entity_id", "entity_name", "editor_name", "editor_role", "user_agent", "ip", "event", "data")
      VALUES ($1, '', '', '', '', '' '', $2, '')`,
			in.GetId(),
			in.GetEvent().String(),
		)
		if err != nil {
			return nil, trace.ErrorAborted(err, h.Log)
		}
	}

	_, err = h.DB.Conn.ExecContext(ctx, sqlQuery)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log, trace.MsgFailedToAdd)
	}

	return response, nil
}
