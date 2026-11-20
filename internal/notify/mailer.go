package notify

import (
	"context"
	"log/slog"
	"time"

	mail "github.com/wneessen/go-mail"

	"github.com/werbot/werbot/internal/platform/config"
)

// SMTPMailer delivers email through an SMTP server.
type SMTPMailer struct {
	client *mail.Client
	from   string
}

// NewSMTPMailer builds a client for cfg: implicit TLS on port 465, mandatory
// STARTTLS otherwise (opportunistic in development, for a local server such
// as Mailpit); credentials only when cfg.Username is set.
func NewSMTPMailer(cfg config.SMTP, development bool) (*SMTPMailer, error) {
	policy := mail.TLSMandatory
	if development {
		policy = mail.TLSOpportunistic
	}
	tls := mail.WithTLSPortPolicy(policy)
	if cfg.Port == 465 {
		tls = mail.WithSSLPort(false)
	}
	opts := []mail.Option{mail.WithPort(cfg.Port), mail.WithTimeout(10 * time.Second), tls}
	if cfg.Username != "" {
		opts = append(opts, mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover), mail.WithUsername(cfg.Username), mail.WithPassword(cfg.Password))
	}
	c, err := mail.NewClient(cfg.Host, opts...)
	if err != nil {
		return nil, err
	}
	return &SMTPMailer{client: c, from: cfg.From}, nil
}

// Send implements Mailer.
func (m *SMTPMailer) Send(ctx context.Context, to, subject, htmlBody string) error {
	msg, err := newMessage(m.from, to, subject, htmlBody)
	if err != nil {
		return err
	}
	return m.client.DialAndSendWithContext(ctx, msg)
}

func newMessage(from, to, subject, html string) (*mail.Msg, error) {
	msg := mail.NewMsg()
	if err := msg.From(from); err != nil {
		return nil, err
	}
	if err := msg.To(to); err != nil {
		return nil, err
	}
	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextHTML, html)
	return msg, nil
}

// LogMailer writes emails to the log instead of sending them.
type LogMailer struct {
	logger      *slog.Logger
	includeBody bool
}

// NewLogMailer returns a LogMailer. includeBody logs the whole email, links
// included, and is for development only.
func NewLogMailer(logger *slog.Logger, includeBody bool) *LogMailer {
	return &LogMailer{logger: logger, includeBody: includeBody}
}

// Send implements Mailer.
func (m *LogMailer) Send(ctx context.Context, to, subject, htmlBody string) error {
	attrs := []any{"to", to, "subject", subject}
	if m.includeBody {
		attrs = append(attrs, "body", htmlBody)
	}
	m.logger.InfoContext(ctx, "email (log mailer)", attrs...)
	return nil
}

// NewMailer returns the SMTP mailer, or the log mailer when cfg.Host is empty.
func NewMailer(cfg config.SMTP, development bool, logger *slog.Logger) (Mailer, error) {
	if cfg.Host == "" {
		logger.Warn("SMTP is not configured: emails are not delivered")
		return NewLogMailer(logger, development), nil
	}
	return NewSMTPMailer(cfg, development)
}
