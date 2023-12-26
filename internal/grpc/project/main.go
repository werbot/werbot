package project

import (
  projectpb "github.com/werbot/werbot/internal/grpc/project/proto"
  "github.com/werbot/werbot/internal/storage/postgres"
  "github.com/werbot/werbot/pkg/logger"
)

var log logger.Logger

// Handler is ...
type Handler struct {
  projectpb.UnimplementedProjectHandlersServer
  DB *postgres.Connect
}

func init() {
  log = logger.New()
}
