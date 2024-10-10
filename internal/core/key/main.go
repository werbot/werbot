package key

import (
	keypb "github.com/werbot/werbot/internal/core/key/proto/key"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/storage/redis"
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
