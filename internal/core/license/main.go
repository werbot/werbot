package license

import (
	licenserpc "github.com/werbot/werbot/internal/core/license/proto/rpc"
	"github.com/werbot/werbot/pkg/logger"
)

var log logger.Logger

// Handler is ...
type Handler struct {
	licenserpc.UnimplementedLicenseHandlersServer
}

func init() {
	log = logger.New()
}
