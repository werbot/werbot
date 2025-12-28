package token

import (
	tokenpb "github.com/werbot/werbot/internal/core/token/proto/rpc"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/worker"
)

var log logger.Logger

// Handler provides token management operations
// Implements gRPC token service handlers for profile, project, and scheme tokens
type Handler struct {
	tokenpb.UnimplementedTokenHandlersServer
	DB     *postgres.Connect
	Worker worker.Client
}

func init() {
	log = logger.New()
}
