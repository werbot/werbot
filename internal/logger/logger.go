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
		log: newLogger.Timestamp().Caller().Logger(),
	}
}

// Info is ...
func (l *Logger) Info() *zerolog.Event {
	return l.log.Info()
}

// Error is ...
func (l *Logger) Error(err error) *zerolog.Event {
	return l.log.Error().Err(err)
}

// Fatal is ...
func (l *Logger) Fatal(err error) *zerolog.Event {
	return l.log.Fatal().Err(err)
}

// FromGRPC is ...
func (l *Logger) FromGRPC(err error) *zerolog.Event {
	code := grpc.Code(err)
	message := grpc.ErrorDesc(err)
	return l.log.Error().
		Str("code", code.String()).
		Interface("error", message)
}
