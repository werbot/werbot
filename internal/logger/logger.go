package logger

import (
	"os"
	"runtime"
	"strings"
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
		log: newLogger.Timestamp().Logger(),
	}
}

// Info is ...
func (l *Logger) Info() *zerolog.Event {
	return l.log.Info()
}

// Error is ...
func (l *Logger) Error(err error) *zerolog.Event {
	return l.errorDetails("error", err)
}

// Fatal is ...
func (l *Logger) Fatal(err error) *zerolog.Event {
	return l.errorDetails("fatal", err)
}

func (l *Logger) errorDetails(level string, err error) *zerolog.Event {
	log := new(zerolog.Event)
	switch level {
	case "error":
		log = l.log.Error()
	case "fatal":
		log = l.log.Fatal()
	}

	pt, file, line, _ := runtime.Caller(2)
	se, _ := status.FromError(err)

	parts := strings.Split(runtime.FuncForPC(pt).Name(), ".")
	pl := len(parts)

	if se.Code() == 2 { // Unknown status
		log = log.Str("status", se.Code().String())
	}

	return log.Str("error", se.Message()).
		Str("file", file).
		Str("function", parts[pl-2]).
		Int("line", line)
}
