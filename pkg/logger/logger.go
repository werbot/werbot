package logger

import (
	"os"
	"sync"

	"github.com/rs/zerolog"
)

// Logger is ...
type Logger struct {
	log zerolog.Logger
}

// New returns a configured logger instance
func New() Logger {
	var logger *zerolog.Logger
	var getLoggerMutex sync.Mutex

	if logger == nil {
		getLoggerMutex.Lock()
		defer getLoggerMutex.Unlock()
		newLogger := zerolog.New(os.Stderr)
		logger = &newLogger
	}

	return Logger{
		log: logger.With().Timestamp().Logger(),
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
