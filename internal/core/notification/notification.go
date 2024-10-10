package notification

import (
	"context"

	"google.golang.org/protobuf/proto"

	messagepb "github.com/werbot/werbot/internal/core/notification/proto/notification"
	"github.com/werbot/werbot/internal/worker"
)

type MetaData map[string]string

// SendMail sends an email based on the request parameters
func (m *Handler) SendMail(ctx context.Context, in *messagepb.SendMail_Request) (*messagepb.SendMail_Response, error) {
	payload, err := proto.Marshal(in)
	if err != nil {
		log.Error(err).Send()
	}

	err = m.Worker.Submit(ctx, worker.TaskSendMail, []byte(payload))
	if err != nil {
		return nil, err
	}

	return &messagepb.SendMail_Response{}, nil
}
