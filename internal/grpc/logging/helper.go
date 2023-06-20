package logging

import (
	loggingpb "github.com/werbot/werbot/internal/grpc/logging/proto"
)

var loggerTable = map[loggingpb.Logger]string{
	loggingpb.Logger_profile: "logs_profile",
	loggingpb.Logger_project: "logs_project",
	loggingpb.Logger_server:  "logs_server",
}
