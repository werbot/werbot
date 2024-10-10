package project

import (
	projectpb "github.com/werbot/werbot/internal/core/project/proto/project"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/storage/postgres"
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
