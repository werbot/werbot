package logger

import (
	"errors"

	"github.com/werbot/werbot/internal/database"
	pb "github.com/werbot/werbot/internal/grpc/proto/logger"
)

// Service is ...
type Service struct {
	db *database.Connect
}

// NewEvent is ...
func NewEvent(db *database.Connect) *Service {
	return &Service{
		db: db,
	}
}

// AddEvent is ...
func (e *Service) AddEvent(logger pb.Logger, event pb.EventType, id int32) error {
	var err error

	switch logger {
	case pb.Logger_profile:
		_, err = e.db.Conn.Exec(`INSERT INTO "logs_profile" ("profile_id", "date", "entity_id", "entity_name", "editor_name", "editor_role", "user_agent", "ip", "event", "data") 
					VALUES ($1, NOW(), '', '', '', '', '' '', $2, '')`,
			id,
			event.String(),
		)

	case pb.Logger_project:
		_, err = e.db.Conn.Exec(`INSERT INTO "logs_project" ("project_id", "date", "entity_id", "entity_name", "editor_name", "editor_role", "user_agent", "ip", "event", "data") 
					VALUES ($1, NOW(), '', '', '', '', '' '', $2, '')`,
			id,
			event.String(),
		)
	}

	if err != nil {
		return errors.New("Problem writing data to database")
	}
	return nil
}
