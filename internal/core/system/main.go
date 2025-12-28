package system

import (
	systemrpc "github.com/werbot/werbot/internal/core/system/proto/rpc"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/storage/postgres"
)

var log logger.Logger

// Handler is ...
type Handler struct {
	systemrpc.UnimplementedSystemHandlersServer
	DB *postgres.Connect
}

func init() {
	log = logger.New()
}
