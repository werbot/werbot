package grpc

import (
	"context"
	"database/sql"

	pb_logging "github.com/werbot/werbot/api/proto/logging"
)

type logging struct {
	pb_logging.UnimplementedLoggingHandlersServer
}

// AddLogRecord is ...
func (l *logging) AddLogRecord(ctx context.Context, in *pb_logging.AddLogRecord_Request) (*pb_logging.AddLogRecord_Response, error) {
	var err error
	var data sql.Result

	switch in.GetLogger() {
	case pb_logging.Logger_profile:
		data, err = service.db.Conn.Exec(`INSERT INTO "logs_profile" ("profile_id", "date", "entity_id", "entity_name", "editor_name", "editor_role", "user_agent", "ip", "event", "data")
					VALUES ($1, NOW(), '', '', '', '', '' '', $2, '')`,
			in.GetId(),
			in.GetEvent().String(),
		)

	case pb_logging.Logger_project:
		data, err = service.db.Conn.Exec(`INSERT INTO "logs_project" ("project_id", "date", "entity_id", "entity_name", "editor_name", "editor_role", "user_agent", "ip", "event", "data")
					VALUES ($1, NOW(), '', '', '', '', '' '', $2, '')`,
			in.GetId(),
			in.GetEvent().String(),
		)
	}
	if err != nil {
		return nil, errFailedToAdd
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errFailedToAdd
	}

	return &pb_logging.AddLogRecord_Response{}, nil
}
