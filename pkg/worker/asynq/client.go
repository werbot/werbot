package asynq

import (
	"context"
	"time"

	"github.com/hibiken/asynq"

	"github.com/werbot/werbot/pkg/worker"
)

type client struct {
	asynqClient *asynq.Client
}

// NewClient is ...
func NewClient(redisURI string) (worker.Client, error) {
	opt, err := asynq.ParseRedisURI(redisURI)
	if err != nil {
		return nil, err
	}

	asynqClient := asynq.NewClient(opt)
	if asynqClient == nil {
		return nil, worker.ErrClientStartFailed
	}

	return &client{asynqClient: asynqClient}, nil
}

// Close is ...
func (c *client) Close() error {
	return c.asynqClient.Close()
}

// Submit is ...
func (c *client) Submit(ctx context.Context, pattern worker.TaskPattern, payload []byte) error {
	return c.submitTask(ctx, pattern, payload)
}

// SubmitDeferred is ...
func (c *client) SubmitDeferred(ctx context.Context, pattern worker.TaskPattern, payload []byte, runAt time.Time) error {
	return c.submitTask(ctx, pattern, payload, asynq.ProcessAt(runAt))
}

// SubmitToBatch is ...
func (c *client) SubmitToBatch(ctx context.Context, pattern worker.TaskPattern, payload []byte) error {
	return c.submitTask(ctx, pattern, payload, asynq.Group(pattern.String()))
}

func (c *client) submitTask(ctx context.Context, pattern worker.TaskPattern, payload []byte, opts ...asynq.Option) error {
	if !pattern.Validate() {
		return worker.ErrTaskPatternInvalid
	}

	task := asynq.NewTask(pattern.String(), payload)
	opts = append(opts, asynq.Queue(pattern.Queue()))
	if _, err := c.asynqClient.EnqueueContext(ctx, task, opts...); err != nil {
		return worker.ErrSubmitFailed
	}

	return nil
}
