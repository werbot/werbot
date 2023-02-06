package member

import (
	"errors"

	memberpb "github.com/werbot/werbot/api/proto/member"
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/storage/postgres"
	"github.com/werbot/werbot/pkg/logger"
)

var (
	log logger.Logger

	errNotFound    = errors.New(internal.MsgNotFound)
	errBadRequest  = errors.New(internal.MsgBadRequest)
	errServerError = errors.New(internal.MsgServerError)

	errFailedToAdd            = errors.New(internal.MsgFailedToAdd)
	errFailedToUpdate         = errors.New(internal.MsgFailedToUpdate)
	errFailedToDelete         = errors.New(internal.MsgFailedToDelete)
	errObjectAlreadyExists    = errors.New(internal.MsgObjectAlreadyExists)
	errInviteIsInvalid        = errors.New(internal.MsgInviteIsInvalid)
	errInviteIsActivated      = errors.New(internal.MsgInviteIsActivated)
	errTransactionCreateError = errors.New(internal.MsgTransactionCreateError)
	errTransactionCommitError = errors.New(internal.MsgTransactionCreateError)
)

// Handler is ...
type Handler struct {
	memberpb.UnimplementedMemberHandlersServer
	DB *postgres.Connect
}

func init() {
	log = logger.New("grpc/member")
}
