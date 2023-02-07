package logging

import (
	"errors"

	"github.com/werbot/werbot/internal"
	loggingpb "github.com/werbot/werbot/internal/grpc/logging/proto"
	"github.com/werbot/werbot/internal/storage/postgres"
	"github.com/werbot/werbot/pkg/logger"
)

var (
	log logger.Logger

	errNotFound    = errors.New(internal.MsgNotFound)
	errBadRequest  = errors.New(internal.MsgBadRequest)
	errServerError = errors.New(internal.MsgServerError)

	errFailedToAdd = errors.New(internal.MsgFailedToAdd)
)

// Handler is ...
type Handler struct {
	loggingpb.UnimplementedLoggingHandlersServer
	DB *postgres.Connect
}

func init() {
	log = logger.New("grpc/logging")
}
