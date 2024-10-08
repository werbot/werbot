package user

import (
	userpb "github.com/werbot/werbot/internal/grpc/user/proto/user"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/worker"
)

var log logger.Logger

// Handler is ...
type Handler struct {
	userpb.UnimplementedUserHandlersServer
	DB     *postgres.Connect
	Worker worker.Client
}

func init() {
	log = logger.New()
}
