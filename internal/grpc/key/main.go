package key

import (
	keypb "github.com/werbot/werbot/internal/grpc/key/proto"
	"github.com/werbot/werbot/internal/storage/postgres"
	"github.com/werbot/werbot/internal/storage/redis"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler is ...
type Handler struct {
	keypb.UnimplementedKeyHandlersServer
	DB    *postgres.Connect
	Redis redis.Handler
	Log   logger.Logger
}
