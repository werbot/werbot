package worker

import (
	"context"

	"google.golang.org/protobuf/proto"

	notificationmessage "github.com/werbot/werbot/internal/core/notification/proto/message"
	notificationenum "github.com/werbot/werbot/internal/core/notification/proto/enum"
	"github.com/werbot/werbot/internal/core/notification/providers/mail"
	"github.com/werbot/werbot/pkg/worker"
)

const (
	TaskSendMail = worker.TaskPattern("send:mail")
)

func sendMail() worker.TaskHandler {
	return func(_ context.Context, payload []byte) error {
		request := &notificationmessage.SendMail_Request{}
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

func tmplMail(tmpl notificationenum.MailTemplate) (template string) {
	templateMap := map[notificationenum.MailTemplate]string{
		notificationenum.MailTemplate_password_reset:                "password-reset",
		notificationenum.MailTemplate_project_invite:                "project-invite",
		notificationenum.MailTemplate_account_deletion_confirmation: "account-deletion-confirmation",
		notificationenum.MailTemplate_account_deletion_info:         "account-deletion-info",
	}

	return templateMap[tmpl]
}
