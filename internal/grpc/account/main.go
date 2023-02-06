package account

import (
	"errors"

	accountpb "github.com/werbot/werbot/api/proto/account"
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/storage/postgres"
	"github.com/werbot/werbot/pkg/logger"
)

var (
	log logger.Logger

	errNotFound    = errors.New(internal.MsgNotFound)
	errBadRequest  = errors.New(internal.MsgBadRequest)
	errServerError = errors.New(internal.MsgServerError)

	errPasswordIsNotValid     = errors.New(internal.MsgPasswordIsNotValid)
	errFailedToUpdate         = errors.New(internal.MsgFailedToUpdate)
	errFailedToAdd            = errors.New(internal.MsgFailedToAdd)
	errHashIsNotValid         = errors.New(internal.MsgHashIsNotValid)
	errTransactionCreateError = errors.New(internal.MsgTransactionCreateError)
	errTransactionCommitError = errors.New(internal.MsgTransactionCreateError)
)

// Handler is ...
type Handler struct {
	accountpb.UnimplementedAccountHandlersServer
	DB *postgres.Connect
}

func init() {
	log = logger.New("grpc/account")
}
