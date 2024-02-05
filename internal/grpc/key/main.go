package key

import (
	keypb "github.com/werbot/werbot/internal/grpc/key/proto"
	"github.com/werbot/werbot/internal/storage/postgres"
	"github.com/werbot/werbot/internal/storage/redis"
	"github.com/werbot/werbot/pkg/logger"
)

var log logger.Logger

// Handler is ...
type Handler struct {
	keypb.UnimplementedKeyHandlersServer
	DB    *postgres.Connect
	Redis *redis.Connect
}

func init() {
	log = logger.New()
}
