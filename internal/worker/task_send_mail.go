package worker

import (
	"context"

	"google.golang.org/protobuf/proto"

	notificationpb "github.com/werbot/werbot/internal/core/notification/proto/notification"
	"github.com/werbot/werbot/internal/core/notification/providers/mail"
	"github.com/werbot/werbot/pkg/worker"
)

const (
	TaskSendMail = worker.TaskPattern("send:mail")
)

func sendMail() worker.TaskHandler {
	return func(_ context.Context, payload []byte) error {
		request := &notificationpb.SendMail_Request{}
		if err := proto.Unmarshal(payload, request); err != nil {
			log.Error(err).Send()
			return err
		}

		template := tmplMail(request.GetTemplate())
		if err := mail.Send(request.GetEmail(), request.GetSubject(), template, request.GetData()); err != nil {
			log.Error(err).Send()
			return err
		}

		return nil
	}
}

func tmplMail(tmpl notificationpb.MailTemplate) (template string) {
	templateMap := map[notificationpb.MailTemplate]string{
		notificationpb.MailTemplate_password_reset:                "password-reset",
		notificationpb.MailTemplate_project_invite:                "project-invite",
		notificationpb.MailTemplate_account_deletion_confirmation: "account-deletion-confirmation",
		notificationpb.MailTemplate_account_deletion_info:         "account-deletion-info",
	}

	return templateMap[tmpl]
}
