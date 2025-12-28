package token

import (
	tokenpb "github.com/werbot/werbot/internal/core/token/proto/rpc"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/worker"
)

var log logger.Logger

// Handler is ...
type Handler struct {
	tokenpb.UnimplementedTokenHandlersServer
	DB     *postgres.Connect
	Worker worker.Client
}

func init() {
	log = logger.New()
}
