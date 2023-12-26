package audit

import (
  auditpb "github.com/werbot/werbot/internal/grpc/audit/proto"
  "github.com/werbot/werbot/internal/storage/postgres"
  "github.com/werbot/werbot/pkg/logger"
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
