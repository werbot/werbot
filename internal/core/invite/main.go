package invite

import (
	invitepb "github.com/werbot/werbot/internal/core/invite/proto/invite"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/storage/postgres"
)

var log logger.Logger

// Handler is ...
type Handler struct {
	invitepb.UnimplementedInviteHandlersServer
	DB *postgres.Connect
}

func init() {
	log = logger.New()
}
