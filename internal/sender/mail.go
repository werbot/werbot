package sender

import (
	"bytes"
	"fmt"
	"html/template"
	"time"

	"github.com/vanng822/go-premailer/premailer"
	mail "github.com/xhit/go-simple-mail/v2"

	"github.com/werbot/werbot/internal/config"
)

// SendMail is ...
func SendMail(to, subject, tmpl string, data any) error {
	tmpls := []string{
		fmt.Sprintf("%s/base.html.tmpl", config.GetString("MAIL_TEMPLATES", "./templates")),
		fmt.Sprintf("%s/%s.html.tmpl", config.GetString("MAIL_TEMPLATES", "./templates"), tmpl),
	}

	t, err := template.New("mail").ParseFiles(tmpls...)
	if err != nil {
		return err
	}

	var tpl bytes.Buffer
	err = t.ExecuteTemplate(&tpl, "base", data)
	if err != nil {
		return err
	}

	//htmlMessage := tpl.String()
	htmlMessage, err := inlineCSS(tpl.String())
	if err != nil {
		return err
	}

	// SMTP Server
	server := mail.NewSMTPClient()
	server.Host = config.GetString("SMTP_HOST", "localhost")
	server.Port = config.GetInt("SMTP_PORT", 25)
	server.Username = config.GetString("SMTP_USERNAME", "")
	server.Password = config.GetString("SMTP_PASSWORD", "")
	server.Encryption = getEncryption(config.GetString("SMTP_ENCRYPTION", "tls"))
	server.KeepAlive = false
	server.ConnectTimeout = 10 * time.Second
	server.SendTimeout = 10 * time.Second

	// SMTP client
	smtpClient, err := server.Connect()
	if err != nil {
		return err
	}

	email := mail.NewMSG()
	email.SetFrom(config.GetString("SMTP_MAIL_FROM", "admin@localhost")).AddTo(to).SetSubject(subject)
	email.SetBody(mail.TextHTML, htmlMessage)

	err = email.Send(smtpClient)
	if err != nil {
		return err
	}

	return nil
}

func inlineCSS(s string) (string, error) {
	options := premailer.Options{
		RemoveClasses:     false,
		CssToAttributes:   false,
		KeepBangImportant: true,
	}

	prem, err := premailer.NewPremailerFromString(s, &options)
	if err != nil {
		return "", err
	}

	html, err := prem.Transform()
	if err != nil {
		return "", err
	}

	return html, nil
}

func getEncryption(s string) mail.Encryption {
	switch s {
	case "tls":
		return mail.EncryptionTLS
	case "ssl":
		return mail.EncryptionSSL
	case "none", "":
		return mail.EncryptionNone
	default:
		return mail.EncryptionTLS
	}
}
