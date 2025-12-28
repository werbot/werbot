package agent

import (
	agentrpc "github.com/werbot/werbot/internal/core/agent/proto/rpc"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/storage/redis"
)

var log logger.Logger

// Handler is ...
type Handler struct {
	agentrpc.UnimplementedAgentHandlersServer
	DB    *postgres.Connect
	Redis *redis.Connect
}

func init() {
	log = logger.New()
}
