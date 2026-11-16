// Package notify renders transactional emails and delivers them through the
// River email queue.
package notify

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"strings"

	"github.com/riverqueue/river"

	"github.com/werbot/werbot/internal/platform/jobs"
)

// Mailer delivers one rendered email.
type Mailer interface {
	Send(ctx context.Context, to, subject, htmlBody string) error
}

// SendEmailArgs is the send_email job. Template is "invite", "password_reset"
// or "account_deletion"; SealedData is the secretbox-sealed JSON of the template
// data, so links with one-time tokens never sit in river_job in clear.
type SendEmailArgs struct {
	To         string `json:"to"`
	Template   string `json:"template"`
	SealedData []byte `json:"sealed_data"`
	KeyID      string `json:"key_id"`
}

// Kind implements river.JobArgs.
func (SendEmailArgs) Kind() string { return "send_email" }

// InsertOpts routes every email job to the email queue.
func (SendEmailArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: jobs.QueueEmail, MaxAttempts: 5}
}

//go:embed templates/*.html
var templateFS embed.FS

var subjects = map[string]func(d map[string]string) string{
	"invite":           func(d map[string]string) string { return d["inviter"] + " invited you to " + d["org"] + " on Werbot" },
	"password_reset":   func(map[string]string) string { return "Reset your Werbot password" },
	"account_deletion": func(map[string]string) string { return "Confirm deletion of your Werbot account" },
}

var templates = func() map[string]*template.Template {
	m := make(map[string]*template.Template, len(subjects))
	for name := range subjects {
		m[name] = template.Must(template.ParseFS(templateFS, "templates/base.html", "templates/"+name+".html")).Option("missingkey=error")
	}
	return m
}()

var lineBreaks = strings.NewReplacer("\r", " ", "\n", " ")

// Render returns the subject and HTML body of template filled with data. HTML
// is escaped; the subject is plain text with line breaks replaced by spaces.
func Render(name string, data map[string]string) (subject, html string, err error) {
	t, ok := templates[name]
	if !ok {
		return "", "", fmt.Errorf("notify: unknown template %q", name)
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", data); err != nil {
		return "", "", fmt.Errorf("notify: render %s: %w", name, err)
	}
	return lineBreaks.Replace(subjects[name](data)), buf.String(), nil
}
