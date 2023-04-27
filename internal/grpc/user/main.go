package user

import (
	userpb "github.com/werbot/werbot/internal/grpc/user/proto"
	"github.com/werbot/werbot/internal/storage/postgres"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler is ...
type Handler struct {
	userpb.UnimplementedUserHandlersServer
	DB  *postgres.Connect
	Log logger.Logger
}
