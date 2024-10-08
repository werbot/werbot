package logger

import (
	"os"

	"github.com/rs/zerolog"
)

// Logger wraps zerolog.Logger to provide custom logging methods.
type Logger struct {
	log zerolog.Logger
}

// New returns a configured Logger instance.
func New() Logger {
	return Logger{
		log: zerolog.New(os.Stderr).With().Timestamp().Logger(),
	}
}

// Info logs an informational message.
func (l *Logger) Info() *zerolog.Event {
	return l.log.Info()
}

// Error logs an error message with caller information.
func (l *Logger) Error(err error) *zerolog.Event {
	return l.log.Error().CallerSkipFrame(1).Caller().Err(err)
}

// Fatal logs a fatal error message with optional error details and caller information.
func (l *Logger) Fatal(err ...error) *zerolog.Event {
	msg := l.log.Fatal()
	if len(err) > 0 {
		return msg.CallerSkipFrame(1).Caller().Err(err[0])
	}
	return msg
}
