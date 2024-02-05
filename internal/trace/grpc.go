package trace

import (
	"database/sql"

	"github.com/werbot/werbot/pkg/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Error is ...
func Error(err error, log logger.Logger, message any) error {
	if err == sql.ErrNoRows {
		return status.Error(codes.NotFound, MsgNotFound)
	}

	dataError := status.Convert(err)
	codeError := dataError.Code()
	msgError := dataError.Message()

	if codeError == codes.Unknown || codeError == codes.Aborted || codeError == codes.Internal {
		log.ErrorGRPC(err, 2).Send()
		msgError = ""
	}

	if msg, ok := message.(string); ok && message != nil {
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
