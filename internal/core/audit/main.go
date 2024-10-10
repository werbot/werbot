package audit

import (
	auditpb "github.com/werbot/werbot/internal/core/audit/proto/audit"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/storage/postgres"
)

var log logger.Logger

// Handler is ...
type Handler struct {
	auditpb.UnimplementedAuditHandlersServer
	DB *postgres.Connect
}

func init() {
	log = logger.New()
}
