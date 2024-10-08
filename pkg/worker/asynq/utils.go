package asynq

import (
	"bytes"
	"context"
	"time"

	"github.com/hibiken/asynq"

	"github.com/werbot/werbot/pkg/worker"
)

type batchConfig struct {
	maxSize     int
	maxDelay    time.Duration
	gracePeriod time.Duration
}

type queues map[string]int

const cronQueue = "cron"

func aggregate(group string, tasks []*asynq.Task) *asynq.Task {
	buf := new(bytes.Buffer)
	for _, t := range tasks {
		buf.Write(t.Payload())
		buf.WriteByte('\n')
	}
	return asynq.NewTask(group+":batch", buf.Bytes())
}

func cronToAsynq(h worker.CronHandler) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, _ *asynq.Task) error {
		return h(ctx)
	}
}

func taskToAsynq(h worker.TaskHandler) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, task *asynq.Task) error {
		return h(ctx, task.Payload())
	}
}
