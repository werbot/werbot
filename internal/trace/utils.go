package trace

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
