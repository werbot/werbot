package worker

import (
	"errors"

	"google.golang.org/grpc"

	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/worker"
	"github.com/werbot/werbot/pkg/worker/asynq"
)

var log = logger.New()

// ErrServerCreateFailed indicates that the worker server could not be created.
var ErrServerCreateFailed = errors.New("failed to create the worker server")

// New creates and starts the worker server. It returns an error when the
// server cannot be created or started so the caller can decide how to react.
func New(redisURI string, grpcClient *grpc.ClientConn) (worker.Server, error) {
	asynqServer := asynq.NewServer(redisURI, asynq.BatchConfig(50, 1, 1))
	if asynqServer == nil {
		return nil, ErrServerCreateFailed
	}

	asynqServer.HandleTask(TaskSendMail, sendMail())

	// asynqServer.HandleTask(task.TaskServerDemo, task.ServerDemo(), asynq.BatchTask())
	// asynqServer.HandleTask(task.TaskServerDemo, task.ServerDemo())

	// cron tasks
	asynqServer.HandleCron("@daily", cronUpdateFirewallList(grpcClient))
	asynqServer.HandleCron("@daily", cronUpdateMMDB())
	asynqServer.HandleCron("@weekly", cronUpdateHAProxyLists())

	if err := asynqServer.Start(); err != nil {
		return nil, err
	}

	return asynqServer, nil
}
