package logging

import (
	loggingpb "github.com/werbot/werbot/internal/grpc/logging/proto"
	"github.com/werbot/werbot/internal/storage/postgres"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler is ...
type Handler struct {
	loggingpb.UnimplementedLoggingHandlersServer
	DB  *postgres.Connect
	Log logger.Logger
}
