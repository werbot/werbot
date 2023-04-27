package trace

import (
	"database/sql"
	"errors"

	"github.com/werbot/werbot/pkg/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Error is ...
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

// ErrorDB is ...
func ErrorDB(err error, log logger.Logger) error {
	if errors.Is(err, sql.ErrNoRows) {
		return Error(codes.NotFound)
	}
	return logAndReturnError(err, codes.Aborted, log)
}

// Aborted indicates the operation was aborted, typically due to a concurrency issue like
// sequencer check failures, transaction aborts, etc.
func ErrorAborted(err error, log logger.Logger, message ...string) error {
	return logAndReturnError(err, codes.Aborted, log, message...)
}

func logAndReturnError(err error, code codes.Code, log logger.Logger, message ...string) error {
	log.ErrorGRPC(status.Error(code, err.Error()), 3).Send()
	return Error(code, message...)
}
