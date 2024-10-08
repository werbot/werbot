package worker

import (
	"context"
	"errors"
	"time"
)

var (
	ErrHandleCronFailed   = errors.New("failed to handle cron")
	ErrServerStartFailed  = errors.New("failed to start the worker server")
	ErrClientStartFailed  = errors.New("failed to start the worker client")
	ErrTaskPatternInvalid = errors.New("task pattern is invalid")
	ErrCronSpecInvalid    = errors.New("cron specification is invalid")
	ErrSubmitFailed       = errors.New("failed to submit the payload")
)

type Server interface {
	HandleTask(pattern TaskPattern, cb TaskHandler, opts ...TaskOption)
	HandleCron(spec CronSpec, cronFunc CronHandler)
	Start() error
	Shutdown()
}

type Client interface {
	Submit(ctx context.Context, pattern TaskPattern, payload []byte) error
	SubmitDeferred(ctx context.Context, pattern TaskPattern, payload []byte, runAt time.Time) error
	SubmitToBatch(ctx context.Context, pattern TaskPattern, payload []byte) error
	Close() error
}
