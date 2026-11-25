package access

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/access/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
)

// RequestCommandApproval asks the approvers of a session's resource to allow
// one command. It comes from the tunnel of gateway instance in.InstanceID,
// without a principal. The command text goes to the approvers' email, never
// to the audit log. An agent run asking again for a command reuses its
// approval (reuseCommand); a person always gets a new one.
func (s *Service) RequestCommandApproval(ctx context.Context, in CommandApprovalInput) (Approval, error) {
	if in.Command == "" {
		return Approval{}, invalid("command is required")
	}
	set, err := s.settings.AccessSettings(ctx, in.OrgID)
	if err != nil {
		return Approval{}, err
	}
	sum := sha256.Sum256([]byte(in.Command))
	a := Approval{OrgID: in.OrgID, RequesterID: in.UserID, Kind: KindCommand, AgentRunID: in.AgentRunID, ResourceID: &in.ResourceID,
		InstanceID: &in.InstanceID, Command: in.Command, CommandSHA256: sum[:], Classes: in.Classes}
	err = s.tx(ctx, in.OrgID, in.UserID, func(st Store, tx pgx.Tx) error {
		reused, ok, err := reuseCommand(ctx, st, a)
		if err != nil || ok {
			a = reused
			return err
		}
		a, err = s.createCommand(ctx, st, tx, a, in.Reasons, set.CommandApprovalTTL)
		return err
	})
	if err != nil {
		return Approval{}, err
	}
	return a, nil
}

// reuseCommand finds the approval an agent run already has for a: pending, or
// approved within AgentApprovalReuse, of the same requester, resource and
// command. The found one moves to a's instance, so its decision reaches the
// asking gateway. ok is false for a person's command or when none is found.
func reuseCommand(ctx context.Context, st Store, a Approval) (_ Approval, ok bool, err error) {
	if a.AgentRunID == nil {
		return a, false, nil
	}
	row, err := st.ReuseCommandApproval(ctx, dbq.ReuseCommandApprovalParams{OrgID: a.OrgID, GatewayInstanceID: *a.InstanceID,
		AgentRunID: *a.AgentRunID, RequesterUserID: a.RequesterID, ResourceID: *a.ResourceID, CommandSha256: a.CommandSHA256,
		ReuseSecs: AgentApprovalReuse.Seconds()})
	if errors.Is(err, pgx.ErrNoRows) {
		return a, false, nil
	}
	if err != nil {
		return a, false, err
	}
	out, err := toApproval(row)
	return out, err == nil, err
}

// createCommand stores a as a new pending command approval of a member on a
// resource of the organization and mails its approvers.
func (s *Service) createCommand(ctx context.Context, st Store, tx pgx.Tx, a Approval, reasons []string, ttl time.Duration) (Approval, error) {
	e, err := loadEvaluator(ctx, st, a.OrgID)
	if err != nil {
		return a, err
	}
	if _, ok := e.members[a.RequesterID]; !ok {
		return a, fmt.Errorf("user not found: %w", apperr.ErrNotFound)
	}
	if len(e.targetsOf(a.ResourceID, nil)) == 0 {
		return a, fmt.Errorf("resource not found: %w", apperr.ErrNotFound)
	}
	t, _ := e.terms(a)
	to := e.approvers(a, t)
	if len(to) == 0 {
		return a, failed("no one can approve this request")
	}
	if t.PolicyID != uuid.Nil {
		a.PolicyID = &t.PolicyID
	}
	if a, err = insert(ctx, st, a, StatusPending, ttl); err != nil {
		return a, err
	}
	if err := auditAs(ctx, tx, a.OrgID, "gateway", &a.RequesterID, "access.request.create", "approval", a.ID,
		map[string]any{"kind": KindCommand, "classes": a.Classes, "reasons": reasons}); err != nil {
		return a, err
	}
	return a, s.mail(ctx, tx, to, "access_request", s.requestMail(e, a, ttl))
}

// GetApproval returns approval id of orgID.
func (s *Service) GetApproval(ctx context.Context, orgID, id uuid.UUID) (Approval, error) {
	var row dbq.Approval
	err := dbx.ReadInOrg(s.pool, orgID, uuid.Nil, func(rd *dbx.Read) (err error) {
		row, err = newStore(rd).GetApproval(ctx, dbq.GetApprovalParams{ID: id, OrgID: orgID})
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Approval{}, fmt.Errorf("approval not found: %w", apperr.ErrNotFound)
	}
	if err != nil {
		return Approval{}, err
	}
	return toApproval(row)
}

// DecidedCommands returns the command approvals of instanceID approved or denied after since, oldest first.
func (s *Service) DecidedCommands(ctx context.Context, orgID, instanceID uuid.UUID, since time.Time) ([]Approval, error) {
	var out []Approval
	err := s.tx(ctx, orgID, uuid.Nil, func(st Store, _ pgx.Tx) error {
		rows, err := st.DecidedCommands(ctx, dbq.DecidedCommandsParams{OrgID: orgID, GatewayInstanceID: instanceID, Since: since})
		if err != nil {
			return err
		}
		for _, r := range rows {
			a, err := toApproval(r)
			if err != nil {
				return err
			}
			out = append(out, a)
		}
		return nil
	})
	return out, err
}
