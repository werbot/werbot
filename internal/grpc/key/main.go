package key

import (
	"errors"

	"github.com/werbot/werbot/internal"
	keypb "github.com/werbot/werbot/internal/grpc/key/proto"
	"github.com/werbot/werbot/internal/storage/cache"
	"github.com/werbot/werbot/internal/storage/postgres"
	"github.com/werbot/werbot/pkg/logger"
)

var (
	log logger.Logger

	errNotFound    = errors.New(internal.MsgNotFound)
	errBadRequest  = errors.New(internal.MsgBadRequest)
	errServerError = errors.New(internal.MsgServerError)

	errPublicKeyIsBroken   = errors.New("the public key has a broken structure")
	errObjectAlreadyExists = errors.New(internal.MsgObjectAlreadyExists)
	errFailedToAdd         = errors.New(internal.MsgFailedToAdd)
	errFailedToUpdate      = errors.New(internal.MsgFailedToUpdate)
	errFailedToDelete      = errors.New(internal.MsgFailedToDelete)
	errIncorrectParameters = errors.New(internal.MsgIncorrectParameters)
)

// Handler is ...
type Handler struct {
	keypb.UnimplementedKeyHandlersServer
	DB    *postgres.Connect
	Cache cache.Cache
}

func init() {
	log = logger.New("grpc/key")
}
