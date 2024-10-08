package worker

import (
	"fmt"

	"google.golang.org/grpc"

	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/worker/asynq"
)

var log logger.Logger

func New(redisURI string, grpcClient *grpc.ClientConn) {
	log = logger.New()

	asynqServer := asynq.NewServer(redisURI, asynq.BatchConfig(50, 1, 1))
	asynqServer.HandleTask(TaskDownloadLicense, downloadLicense())
	asynqServer.HandleTask(TaskSendMail, sendMail())

	// asynqServer.HandleTask(task.TaskServerDemo, task.ServerDemo(), asynq.BatchTask())
	// asynqServer.HandleTask(task.TaskServerDemo, task.ServerDemo())

	// cron tasks
	asynqServer.HandleCron("@daily", cronUpdateFirewallList(grpcClient))
	asynqServer.HandleCron("@daily", cronUpdateMMDB())
	asynqServer.HandleCron("@weekly", cronUpdateHAProxyLists())

	if err := asynqServer.Start(); err != nil {
		fmt.Print(err)
	}
}
