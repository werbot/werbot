package user

import (
	"errors"

	"github.com/werbot/werbot/internal"
	userpb "github.com/werbot/werbot/internal/grpc/user/proto"
	"github.com/werbot/werbot/internal/storage/postgres"
	"github.com/werbot/werbot/pkg/logger"
)

var (
	log logger.Logger

	errNotFound    = errors.New(internal.MsgNotFound)
	errBadRequest  = errors.New(internal.MsgBadRequest)
	errServerError = errors.New(internal.MsgServerError)

	errTransactionCreateError = errors.New(internal.MsgTransactionCreateError)
	errObjectAlreadyExists    = errors.New(internal.MsgObjectAlreadyExists)
	errFailedToAdd            = errors.New(internal.MsgFailedToAdd)
	errTransactionCommitError = errors.New(internal.MsgTransactionCreateError)
	errPasswordIsNotValid     = errors.New(internal.MsgPasswordIsNotValid)
	errFailedToUpdate         = errors.New(internal.MsgFailedToUpdate)
	errTokenIsNotValid        = errors.New(internal.MsgTokenIsNotValid)
	errHashIsNotValid         = errors.New(internal.MsgHashIsNotValid)
)

// Handler is ...
type Handler struct {
	userpb.UnimplementedUserHandlersServer
	DB *postgres.Connect
}

func init() {
	log = logger.New("grpc/user")
}
