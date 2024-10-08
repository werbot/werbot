package worker

import (
	"context"
	"strings"
)

type TaskOption func(t *Task)

type Task struct {
	Pattern TaskPattern
	Handler TaskHandler
}

type TaskHandler func(ctx context.Context, payload []byte) error

type TaskPattern string

func (tp TaskPattern) String() string {
	return string(tp)
}

func (tp TaskPattern) Validate() bool {
	return len(strings.Split(string(tp), ":")) == 2
}

func (tp TaskPattern) MustValidate() {
	if !tp.Validate() {
		panic("invalid task pattern: " + tp)
	}
}

func (tp TaskPattern) Queue() string {
	return strings.Split(string(tp), ":")[0]
}
