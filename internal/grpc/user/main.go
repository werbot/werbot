package user

import (
  userpb "github.com/werbot/werbot/internal/grpc/user/proto"
  "github.com/werbot/werbot/internal/storage/postgres"
  "github.com/werbot/werbot/pkg/logger"
)

var log logger.Logger

// Handler is ...
type Handler struct {
  userpb.UnimplementedUserHandlersServer
  DB *postgres.Connect
}

func init() {
  log = logger.New()
}
