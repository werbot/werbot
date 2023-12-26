package server

import (
  serverpb "github.com/werbot/werbot/internal/grpc/server/proto"
  "github.com/werbot/werbot/internal/storage/postgres"
  "github.com/werbot/werbot/internal/storage/redis"
  "github.com/werbot/werbot/pkg/logger"
)

var log logger.Logger

// Handler is ...
type Handler struct {
  serverpb.UnimplementedServerHandlersServer
  DB    *postgres.Connect
  Redis redis.Handler
}

func init() {
  log = logger.New()
}
