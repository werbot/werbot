package project

import (
	projectrpc "github.com/werbot/werbot/internal/core/project/proto/rpc"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/storage/postgres"
)

var log logger.Logger

// Handler is ...
type Handler struct {
	projectrpc.UnimplementedProjectHandlersServer
	DB *postgres.Connect
}

func init() {
	log = logger.New()
}
