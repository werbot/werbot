package project

import (
	"errors"

	projectpb "github.com/werbot/werbot/api/proto/project"
	"github.com/werbot/werbot/internal"
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
	errFailedToDelete         = errors.New(internal.MsgFailedToDelete)
)

// Handler is ...
type Handler struct {
	projectpb.UnimplementedProjectHandlersServer
	DB *postgres.Connect
}

func init() {
	log = logger.New("grpc/project")
}
