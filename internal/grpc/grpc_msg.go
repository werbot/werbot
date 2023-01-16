package grpc

import (
	"errors"

	"github.com/werbot/werbot/internal"
)

var (
	errNotFound    = errors.New(internal.MsgNotFound)
	errBadRequest  = errors.New(internal.MsgBadRequest)
	errServerError = errors.New(internal.MsgServerError)

	errIncorrectParameters    = errors.New(internal.MsgIncorrectParameters)
	errPasswordIsNotValid     = errors.New(internal.MsgPasswordIsNotValid)
	errAccessIsDenied         = errors.New(internal.MsgAccessIsDenied)
	errAccessIsDeniedUser     = errors.New(internal.MsgAccessIsDeniedUser)
	errAccessIsDeniedTime     = errors.New(internal.MsgAccessIsDeniedTime)
	errAccessIsDeniedCountry  = errors.New(internal.MsgAccessIsDeniedCountry)
	errAccessIsDeniedIP       = errors.New(internal.MsgAccessIsDeniedIP)
	errObjectAlreadyExists    = errors.New(internal.MsgObjectAlreadyExists)
	errFailedToOpenFile       = errors.New(internal.MsgFailedToOpenFile)
	errFailedToAdd            = errors.New(internal.MsgFailedToAdd)
	errFailedToUpdate         = errors.New(internal.MsgFailedToUpdate)
	errFailedToDelete         = errors.New(internal.MsgFailedToDelete)
	errInviteIsInvalid        = errors.New(internal.MsgInviteIsInvalid)
	errInviteIsActivated      = errors.New(internal.MsgInviteIsActivated)
	errTransactionCreateError = errors.New(internal.MsgTransactionCreateError)
	errTransactionCommitError = errors.New(internal.MsgTransactionCreateError)
	errTokenIsNotValid        = errors.New(internal.MsgTokenIsNotValid)
	errHashIsNotValid         = errors.New(internal.MsgHashIsNotValid)
)
