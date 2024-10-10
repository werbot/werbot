package scheme

import (
	schemepb "github.com/werbot/werbot/internal/core/scheme/proto/scheme"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/storage/redis"
)

var log logger.Logger

// Handler is ...
type Handler struct {
	schemepb.UnimplementedSchemeHandlersServer
	DB    *postgres.Connect
	Redis *redis.Connect
}

func init() {
	log = logger.New()
}
