package grpc

import (
	"context"
	"database/sql"

	loggingpb "github.com/werbot/werbot/api/proto/logging"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
)

type logging struct {
	loggingpb.UnimplementedLoggingHandlersServer
}

// TODO ListRecords is ...
func (l *logging) ListRecords(ctx context.Context, in *loggingpb.ListRecords_Request) (*loggingpb.ListRecords_Response, error) {
	response := new(loggingpb.ListRecords_Response)
	return response, nil
}

// TODO Record is ...
func (l *logging) Record(ctx context.Context, in *loggingpb.Record_Request) (*loggingpb.Record_Response, error) {
	response := new(loggingpb.Record_Response)
	return response, nil
}

// AddLogRecord is ...
func (l *logging) AddLogRecord(ctx context.Context, in *loggingpb.AddRecord_Request) (*loggingpb.AddRecord_Response, error) {
	var sqlQuery string
	var data sql.Result
	var err error
	response := new(loggingpb.AddRecord_Response)

	switch in.GetLogger() {
	case loggingpb.Logger_profile:
		sqlQuery, err = sanitize.SQL(`INSERT INTO "logs_profile" ("profile_id", "entity_id", "entity_name", "editor_name", "editor_role", "user_agent", "ip", "event", "created", "data")
      VALUES ($1, '', '', '', '', '' '', $2, NOW(), '')`,
			in.GetId(),
			in.GetEvent().String(),
		)
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errBadRequest
		}

	case loggingpb.Logger_project:
		sqlQuery, err = sanitize.SQL(`INSERT INTO "logs_project" ("project_id", "entity_id", "entity_name", "editor_name", "editor_role", "user_agent", "ip", "event", "created", "data")
      VALUES ($1, '', '', '', '', '' '', $2, NOW(), '')`,
			in.GetId(),
			in.GetEvent().String(),
		)
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errBadRequest
		}
	}

	if data, err = service.db.Conn.Exec(sqlQuery); err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errFailedToAdd
	}

	return response, nil
}
