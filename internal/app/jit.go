package app

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/notify"
)

// accessEmails is the email queue of access. It reads s.Jobs at call time:
// access is built before the River client that needs its workers.
type accessEmails struct{ s *services }

func (a accessEmails) EnqueueTx(ctx context.Context, tx pgx.Tx, to, template string, data map[string]string) error {
	return notify.NewQueue(a.s.Jobs, a.s.Box).EnqueueTx(ctx, tx, to, template, data)
}

// accessSettings is identity's organization settings as access.Settings. It
// reads s.Identity at call time, which is set before any request is served.
type accessSettings struct{ s *services }

func (a accessSettings) AccessSettings(ctx context.Context, orgID uuid.UUID) (access.Settings, error) {
	o, err := a.s.Identity.Settings(ctx, orgID)
	if err != nil {
		return access.Settings{}, err
	}
	return access.Settings{GrantDefaultTTL: o.GrantDefaultTTL, GrantMaxTTL: o.GrantMaxTTL,
		RequestPendingTTL: o.RequestPendingTTL, CommandApprovalTTL: o.CommandApprovalTTL}, nil
}
