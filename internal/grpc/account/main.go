package account

import (
	accountpb "github.com/werbot/werbot/internal/grpc/account/proto"
	"github.com/werbot/werbot/internal/storage/postgres"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler is ...
type Handler struct {
	accountpb.UnimplementedAccountHandlersServer
	DB  *postgres.Connect
	Log logger.Logger
}
