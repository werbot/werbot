package event

import (
	eventpb "github.com/werbot/werbot/internal/grpc/event/proto"
	"github.com/werbot/werbot/internal/storage/postgres"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler is ...
type Handler struct {
	eventpb.UnimplementedEventHandlersServer
	DB  *postgres.Connect
	Log logger.Logger
}
