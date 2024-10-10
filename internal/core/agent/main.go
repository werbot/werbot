package agent

import (
	agentpb "github.com/werbot/werbot/internal/core/agent/proto/agent"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/storage/redis"
)

var log logger.Logger

// Handler is ...
type Handler struct {
	agentpb.UnimplementedAgentHandlersServer
	DB    *postgres.Connect
	Redis *redis.Connect
}

func init() {
	log = logger.New()
}
