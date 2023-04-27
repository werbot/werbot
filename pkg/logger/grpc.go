package logger

import (
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrorGRPC is show errors from GRPC
func (l *Logger) ErrorGRPC(err error, frm ...int) *zerolog.Event {
	frame := 1
	if len(frm) > 0 {
		frame = frm[0]
	}

	dataError := status.Convert(err)
	switch dataError.Code() {
	case codes.Internal, codes.Unknown, codes.Aborted, codes.Canceled, codes.DeadlineExceeded, codes.ResourceExhausted:
		return l.log.Error().CallerSkipFrame(frame).Caller().
			Str("code", dataError.Code().String()).
			Str("error", dataError.Message())
	}

	return nil
}
