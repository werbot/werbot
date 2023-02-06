package utility

import (
	"errors"

	utilitypb "github.com/werbot/werbot/api/proto/utility"
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/storage/postgres"
	"github.com/werbot/werbot/pkg/logger"
)

var (
	log logger.Logger

	errNotFound    = errors.New(internal.MsgNotFound)
	errBadRequest  = errors.New(internal.MsgBadRequest)
	errServerError = errors.New(internal.MsgServerError)

	errFailedToOpenFile = errors.New(internal.MsgFailedToOpenFile)
	errAccessIsDenied   = errors.New(internal.MsgAccessIsDenied)
)

// Handler is ...
type Handler struct {
	utilitypb.UnimplementedUtilityHandlersServer
	DB *postgres.Connect
}

func init() {
	log = logger.New("grpc/utility")
}
