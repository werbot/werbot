package notification

import (
	messagepb "github.com/werbot/werbot/internal/core/notification/proto/notification"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/worker"
)

var log logger.Logger

// Handler is ...
type Handler struct {
	messagepb.UnimplementedNotificationHandlersServer
	DB     *postgres.Connect
	Worker worker.Client
}

func init() {
	log = logger.New()
}
