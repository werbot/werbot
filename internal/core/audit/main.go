package audit

import (
	auditrpc "github.com/werbot/werbot/internal/core/audit/proto/rpc"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/storage/postgres"
)

var log logger.Logger

// Handler is ...
type Handler struct {
	auditrpc.UnimplementedAuditHandlersServer
	DB *postgres.Connect
}

func init() {
	log = logger.New()
}
