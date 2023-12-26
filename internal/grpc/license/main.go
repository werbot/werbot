package license

import (
  licensepb "github.com/werbot/werbot/internal/grpc/license/proto"
  "github.com/werbot/werbot/pkg/logger"
)

var log logger.Logger

// Handler is ...
type Handler struct {
  licensepb.UnimplementedLicenseHandlersServer
}

func init() {
  log = logger.New()
}
