package member

import (
	memberpb "github.com/werbot/werbot/internal/grpc/member/proto"
	"github.com/werbot/werbot/internal/storage/postgres"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler is ...
type Handler struct {
	memberpb.UnimplementedMemberHandlersServer
	DB  *postgres.Connect
	Log logger.Logger
}
