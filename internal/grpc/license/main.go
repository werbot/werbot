package license

import (
	licensepb "github.com/werbot/werbot/internal/grpc/license/proto"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler is ...
type Handler struct {
	licensepb.UnimplementedLicenseHandlersServer
	Log logger.Logger
}
