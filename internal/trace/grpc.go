package trace

import (
	"database/sql"

	"github.com/werbot/werbot/pkg/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Error is takes in a code of the type codes.Code and a variadic parameter message of type string.
func Error(code codes.Code, message ...string) error {
	errorMessage := MsgUnknownError

	switch code {
	case codes.InvalidArgument:
		errorMessage = MsgInvalidArgument
	case codes.NotFound:
		errorMessage = MsgNotFound
	case codes.AlreadyExists:
		errorMessage = MsgAlreadyExists
	case codes.PermissionDenied:
		errorMessage = MsgPermissionDenied
	case codes.Aborted:
		errorMessage = MsgAborted
	}

	if len(message) > 0 {
		errorMessage = message[0]
	}

	return status.Error(code, errorMessage)
}

// Aborted indicates the operation was aborted, typically due to a concurrency issue like
// sequencer check failures, transaction aborts, etc.
func ErrorAborted(err error, log logger.Logger, message ...string) error {
	log.ErrorGRPC(status.Error(codes.Aborted, err.Error()), 2).Send()

	if err == sql.ErrNoRows {
		return Error(codes.NotFound, message...)
	}

	return Error(codes.Aborted, message...)
}
