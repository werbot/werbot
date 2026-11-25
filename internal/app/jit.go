package app

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/notify"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/policy"
	"github.com/werbot/werbot/internal/tunnel"
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

// policyGrants is the active grants of access as a policy.GrantSource.
type policyGrants struct{ acc *access.Service }

func (p policyGrants) PolicyGrants(ctx context.Context, orgID uuid.UUID) ([]policy.SnapshotGrant, error) {
	gs, err := p.acc.ActiveGrants(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]policy.SnapshotGrant, len(gs))
	for i, g := range gs {
		out[i] = policy.SnapshotGrant{ID: g.ID, UserID: g.UserID, ResourceID: g.ResourceID, ExpiresUnix: g.ExpiresAt.Unix()}
	}
	return out, nil
}

// policyLocks is the active locks of access as a policy.LockSource.
type policyLocks struct{ acc *access.Service }

func (p policyLocks) PolicyLocks(ctx context.Context, orgID uuid.UUID) ([]policy.SnapshotLock, error) {
	ls, err := p.acc.ActiveLocks(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]policy.SnapshotLock, len(ls))
	for i, l := range ls {
		out[i] = policy.SnapshotLock{ID: l.ID, TargetType: l.TargetType, TargetID: l.TargetID}
		if l.ExpiresAt != nil {
			// round up: a lock never ends before its expires_at (grants round down)
			out[i].ExpiresUnix = l.ExpiresAt.Add(time.Second - time.Nanosecond).Unix()
		}
	}
	return out, nil
}

// tunnelApprovals is the command approvals of access as tunnel.Approvals.
type tunnelApprovals struct{ acc *access.Service }

func (t tunnelApprovals) Request(ctx context.Context, r tunnel.ApprovalRequest) (tunnel.Approval, error) {
	a, err := t.acc.RequestCommandApproval(ctx, access.CommandApprovalInput{OrgID: r.OrgID, UserID: r.UserID, ResourceID: r.ResourceID,
		InstanceID: r.InstanceID, AgentRunID: r.AgentRunID, Command: r.Command, Classes: r.Classes, Reasons: r.Reasons})
	if err != nil {
		return tunnel.Approval{}, err
	}
	return t.approval(a), nil
}

// Get hides the approvals of other instances behind ErrNotFound.
func (t tunnelApprovals) Get(ctx context.Context, orgID, instanceID, id uuid.UUID) (tunnel.Approval, error) {
	a, err := t.acc.GetApproval(ctx, orgID, id)
	if err != nil {
		return tunnel.Approval{}, err
	}
	if a.Kind != access.KindCommand || a.InstanceID == nil || *a.InstanceID != instanceID {
		return tunnel.Approval{}, fmt.Errorf("approval not found: %w", apperr.ErrNotFound)
	}
	return t.approval(a), nil
}

func (t tunnelApprovals) DecidedSince(ctx context.Context, orgID, instanceID uuid.UUID, since time.Time) ([]tunnel.Approval, error) {
	as, err := t.acc.DecidedCommands(ctx, orgID, instanceID, since)
	if err != nil {
		return nil, err
	}
	out := make([]tunnel.Approval, len(as))
	for i, a := range as {
		out[i] = t.approval(a)
	}
	return out, nil
}

func (t tunnelApprovals) approval(a access.Approval) tunnel.Approval {
	return tunnel.Approval{ID: a.ID, Status: a.Status, ExpiresAt: a.ExpiresAt, URL: t.acc.RequestURL(a.ID)}
}
