package event

import (
	eventpb "github.com/werbot/werbot/internal/core/event/proto/event"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/storage/postgres"
)

var log logger.Logger

// Handler is ...
type Handler struct {
	eventpb.UnimplementedEventHandlersServer
	DB *postgres.Connect
}

func init() {
	log = logger.New()
}
