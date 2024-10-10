package system

import (
	systempb "github.com/werbot/werbot/internal/core/system/proto/system"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/storage/postgres"
)

var log logger.Logger

// Handler is ...
type Handler struct {
	systempb.UnimplementedSystemHandlersServer
	DB *postgres.Connect
}

func init() {
	log = logger.New()
}
