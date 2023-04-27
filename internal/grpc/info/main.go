package info

import (
	infopb "github.com/werbot/werbot/internal/grpc/info/proto"
	"github.com/werbot/werbot/internal/storage/postgres"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler is ...
type Handler struct {
	infopb.UnimplementedInfoHandlersServer
	DB  *postgres.Connect
	Log logger.Logger
}
