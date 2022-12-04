package logger

import (
	"os"
	"sync"

	"github.com/rs/zerolog"
)

// New returns a configured logger instance
func New(module string) zerolog.Logger {
	var logger *zerolog.Logger
	var getLoggerMutex sync.Mutex

	if logger == nil {
		getLoggerMutex.Lock()
		defer getLoggerMutex.Unlock()
		newLogger := zerolog.New(os.Stderr)
		logger = &newLogger
	}
	if module != "" {
		return logger.With().Str("module", module).Timestamp().Logger()
	}
	return logger.With().Timestamp().Logger()
}
