package logger

import (
	"os"
	"sync"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/status"
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

// ErrorGRPC is ...
func (l *Logger) ErrorGRPC(err error) *zerolog.Event {
	log := l.log.Error()
	se, ok := status.FromError(err)
	if !ok {
		return log.
			Str("error", se.Message()).
			Str("status", se.Code().String())
	}
	return log.Str("error", err.Error())
}
