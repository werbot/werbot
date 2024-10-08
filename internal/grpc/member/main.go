package member

import (
	memberpb "github.com/werbot/werbot/internal/grpc/member/proto/member"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/worker"
)

var log logger.Logger

// Handler is ...
type Handler struct {
	memberpb.UnimplementedMemberHandlersServer
	DB     *postgres.Connect
	Worker worker.Client
}

func init() {
	log = logger.New()
}
