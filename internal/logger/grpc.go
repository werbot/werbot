package logger

import (
	"runtime"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/status"
)

// OutErrorLog is ...
func OutErrorLog(component string, err error, message string) {
	// pt, file, line, _ := runtime.Caller(1)
	pt, _, _, _ := runtime.Caller(1)
	se, _ := status.FromError(err)
	log.Error().Err(err).
		Str("component", component).
		Str("function", runtime.FuncForPC(pt).Name()).
		// Str("file", file).
		// Int("line", line).
		Str("status", se.Code().String()).
		Str("message", se.Message()).
		Msg(message)
}
