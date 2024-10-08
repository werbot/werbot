package asynq

import "github.com/werbot/werbot/pkg/worker"

func BatchTask() worker.TaskOption {
	return func(t *worker.Task) {
		t.Pattern += ":batch"
	}
}
