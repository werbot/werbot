package license

import (
	licensepb "github.com/werbot/werbot/api/proto/license"
	"github.com/werbot/werbot/pkg/logger"
)

var log logger.Logger

// Handler is ...
type Handler struct {
	licensepb.UnimplementedLicenseHandlersServer
}

func init() {
	log = logger.New("grpc/license")
}
