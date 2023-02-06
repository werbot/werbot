package firewall

import (
	"errors"

	firewallpb "github.com/werbot/werbot/api/proto/firewall"
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/storage/postgres"
	"github.com/werbot/werbot/pkg/logger"
)

var (
	log logger.Logger

	errNotFound    = errors.New(internal.MsgNotFound)
	errBadRequest  = errors.New(internal.MsgBadRequest)
	errServerError = errors.New(internal.MsgServerError)

	errIncorrectParameters   = errors.New(internal.MsgIncorrectParameters)
	errAccessIsDeniedCountry = errors.New(internal.MsgAccessIsDeniedCountry)
	errObjectAlreadyExists   = errors.New(internal.MsgObjectAlreadyExists)
	errFailedToAdd           = errors.New(internal.MsgFailedToAdd)
	errFailedToDelete        = errors.New(internal.MsgFailedToDelete)
	errAccessIsDeniedUser    = errors.New(internal.MsgAccessIsDeniedUser)
	errAccessIsDeniedIP      = errors.New(internal.MsgAccessIsDeniedIP)
	errAccessIsDeniedTime    = errors.New(internal.MsgAccessIsDeniedTime)
	errFailedToUpdate        = errors.New(internal.MsgFailedToUpdate)
)

// Handler is ...
type Handler struct {
	firewallpb.UnimplementedFirewallHandlersServer
	DB *postgres.Connect
}

func init() {
	log = logger.New("grpc/firewall")
}
