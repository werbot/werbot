package logger

import (
	"os"
	"sync"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
)

// Logger is ...
type Logger struct {
	log zerolog.Logger
}

// New returns a configured logger instance
func New(module string) Logger {
	var logger *zerolog.Logger
	var getLoggerMutex sync.Mutex

	if logger == nil {
		getLoggerMutex.Lock()
		defer getLoggerMutex.Unlock()
		newLogger := zerolog.New(os.Stderr)
		logger = &newLogger
	}

	newLogger := logger.With()
	if module != "" {
		newLogger = newLogger.Str("module", module)
	}

	return Logger{
		log: newLogger.Timestamp().Logger(),
	}
}

// Info is ...
func (l *Logger) Info() *zerolog.Event {
	return l.log.Info()
}

// Error is ...
func (l *Logger) Error(err error) *zerolog.Event {
	return l.log.Error().CallerSkipFrame(1).Caller().Err(err)
}

// Fatal is ...
func (l *Logger) Fatal(err error) *zerolog.Event {
	return l.log.Fatal().CallerSkipFrame(1).Caller().Err(err)
}

// FromGRPC is ...
func (l *Logger) FromGRPC(err error, frame ...int) *zerolog.Event {
	code := grpc.Code(err)
	message := grpc.ErrorDesc(err)
	_frame := 1
	if len(frame) > 0 {
		_frame = frame[0]
	}
	return l.log.Error().CallerSkipFrame(_frame).Caller().
		Str("code", code.String()).
		Interface("error", message)
}
