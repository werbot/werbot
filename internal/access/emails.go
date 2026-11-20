package access

import (
	"context"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// RequestURL is the web page of request id.
func (s *Service) RequestURL(id uuid.UUID) string {
	return s.cfg.WebURL + "/requests?id=" + id.String()
}

// resourceText names what a asks for: the resource name or the selector "k=v,…".
func (e evaluator) resourceText(a Approval) string {
	if a.ResourceID != nil {
		return e.names[*a.ResourceID]
	}
	pairs := make([]string, 0, len(a.Selector))
	for _, k := range slices.Sorted(maps.Keys(a.Selector)) {
		pairs = append(pairs, k+"="+a.Selector[k])
	}
	return strings.Join(pairs, ",")
}

// formatDuration writes d without zero minutes and seconds: 2h, 30m, 1h30m.
func formatDuration(d time.Duration) string {
	s := d.String()
	if d%time.Minute == 0 {
		s = strings.TrimSuffix(s, "0s")
	}
	if d%time.Hour == 0 {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}

// mail queues template with data to every member of to.
func (s *Service) mail(ctx context.Context, tx pgx.Tx, to []member, template string, data map[string]string) error {
	for _, m := range to {
		if err := s.emails.EnqueueTx(ctx, tx, m.Email, template, data); err != nil {
			return err
		}
	}
	return nil
}

// requestMail is the access_request email of a; command is the text of a command approval.
func (s *Service) requestMail(e evaluator, a Approval, wait time.Duration) map[string]string {
	d := a.Duration
	if a.Kind == KindCommand {
		d = wait
	}
	return map[string]string{"url": s.RequestURL(a.ID), "requester": e.members[a.RequesterID].Login,
		"resource": e.resourceText(a), "duration": formatDuration(d), "reason": a.Reason, "command": a.Command}
}
