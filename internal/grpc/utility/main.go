package utility

import (
	utilitypb "github.com/werbot/werbot/internal/grpc/utility/proto"
	"github.com/werbot/werbot/internal/storage/postgres"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler is ...
type Handler struct {
	utilitypb.UnimplementedUtilityHandlersServer
	DB  *postgres.Connect
	Log logger.Logger
}
