package trace

import (
	"database/sql"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/werbot/werbot/pkg/logger"
)

// Error handles errors and logs specific cases.
func Error(err error, log logger.Logger, message any) error {
	if errors.Is(err, sql.ErrNoRows) {
		msgError := MsgNotFound
		return status.Error(codes.NotFound, msgError)
	}

	dataError := status.Convert(err)
	codeError := dataError.Code()
	msgError := dataError.Message()

	switch codeError {
	case codes.Unknown, codes.Aborted, codes.Internal:
		log.ErrorGRPC(err, 2).Send()
		msgError = ""
	}

	if msg, ok := message.(string); ok && msg != "" {
		msgError = msg
	}

	return status.Error(codeError, msgError)
}

// The following code defines a struct named ErrorInfo.
// This struct contains two fields, Code and Message, both of type string.
type ErrorInfo struct {
	Code    codes.Code
	Message string
}

// ParseError converts an error into an ErrorInfo struct.
// If the error is nil, it returns nil.
func ParseError(err error) *ErrorInfo {
	if err == nil {
		return nil
	}

	dataError := status.Convert(err)
	return &ErrorInfo{
		Code:    dataError.Code(),
		Message: dataError.Message(),
	}
}
