package server

import (
	"errors"

	"github.com/werbot/werbot/internal"
	serverpb "github.com/werbot/werbot/internal/grpc/server/proto"
	"github.com/werbot/werbot/internal/storage/cache"
	"github.com/werbot/werbot/internal/storage/postgres"
	"github.com/werbot/werbot/pkg/logger"
)

var (
	log logger.Logger

	errNotFound    = errors.New(internal.MsgNotFound)
	errBadRequest  = errors.New(internal.MsgBadRequest)
	errServerError = errors.New(internal.MsgServerError)

	errTransactionCreateError = errors.New(internal.MsgTransactionCreateError)
	errFailedToAdd            = errors.New(internal.MsgFailedToAdd)
	errTransactionCommitError = errors.New(internal.MsgTransactionCreateError)
	errFailedToUpdate         = errors.New(internal.MsgFailedToUpdate)
	errFailedToDelete         = errors.New(internal.MsgFailedToDelete)
)

// Handler is ...
type Handler struct {
	serverpb.UnimplementedServerHandlersServer
	DB    *postgres.Connect
	Cache cache.Cache
}

func init() {
	log = logger.New("grpc/server")
}
