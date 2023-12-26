package member

import (
  memberpb "github.com/werbot/werbot/internal/grpc/member/proto"
  "github.com/werbot/werbot/internal/storage/postgres"
  "github.com/werbot/werbot/pkg/logger"
)

var log logger.Logger

// Handler is ...
type Handler struct {
  memberpb.UnimplementedMemberHandlersServer
  DB *postgres.Connect
}

func init() {
  log = logger.New()
}
