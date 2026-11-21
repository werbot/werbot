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
// to the audit log.
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
		e, err := loadEvaluator(ctx, st, in.OrgID)
		if err != nil {
			return err
		}
		if _, ok := e.members[in.UserID]; !ok {
			return fmt.Errorf("user not found: %w", apperr.ErrNotFound)
		}
		if len(e.targetsOf(a.ResourceID, nil)) == 0 {
			return fmt.Errorf("resource not found: %w", apperr.ErrNotFound)
		}
		t, _ := e.terms(a)
		to := e.approvers(a, t)
		if len(to) == 0 {
			return failed("no one can approve this request")
		}
		if t.PolicyID != uuid.Nil {
			a.PolicyID = &t.PolicyID
		}
		if a, err = insert(ctx, st, a, StatusPending, set.CommandApprovalTTL); err != nil {
			return err
		}
		if err := auditAs(ctx, tx, in.OrgID, "gateway", &in.UserID, "access.request.create", "approval", a.ID,
			map[string]any{"kind": KindCommand, "classes": in.Classes, "reasons": in.Reasons}); err != nil {
			return err
		}
		return s.mail(ctx, tx, to, "access_request", s.requestMail(e, a, set.CommandApprovalTTL))
	})
	if err != nil {
		return Approval{}, err
	}
	return a, nil
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
