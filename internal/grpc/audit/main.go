package audit

import (
	"errors"

	"github.com/werbot/werbot/internal"
	auditpb "github.com/werbot/werbot/internal/grpc/audit/proto"
	"github.com/werbot/werbot/internal/storage/postgres"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler is ...
type Handler struct {
	auditpb.UnimplementedAuditHandlersServer
	DB *postgres.Connect
}

var (
	log logger.Logger

	errNotFound    = errors.New(internal.MsgNotFound)
	errBadRequest  = errors.New(internal.MsgBadRequest)
	errServerError = errors.New(internal.MsgServerError)

	errIncorrectParameters = errors.New(internal.MsgIncorrectParameters)
	errFailedToAdd         = errors.New(internal.MsgFailedToAdd)
	errFailedToUpdate      = errors.New(internal.MsgFailedToUpdate)
)

func init() {
	log = logger.New("grpc/audit")
}
