package access

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/access/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// Decide approves or denies a pending request under the current policies and
// resources. An approved access request grants every resource it covers now;
// a decided command is announced to its gateway instance.
func (s *Service) Decide(ctx context.Context, p principal.Principal, id uuid.UUID, approve bool, comment string) (Approval, error) {
	comment, err := text("comment", comment)
	if err != nil {
		return Approval{}, err
	}
	set, err := s.settings.AccessSettings(ctx, p.OrgID)
	if err != nil {
		return Approval{}, err
	}
	var out Approval
	err = s.tx(ctx, p.OrgID, p.UserID, func(st Store, tx pgx.Tx) error {
		row, err := st.GetApproval(ctx, dbq.GetApprovalParams{ID: id, OrgID: p.OrgID})
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("request not found: %w", apperr.ErrNotFound)
		}
		if err != nil {
			return err
		}
		a, err := toApproval(row)
		if err != nil {
			return err
		}
		e, err := loadEvaluator(ctx, st, p.OrgID)
		if err != nil {
			return err
		}
		if !e.visible(a, p.UserID) {
			return fmt.Errorf("request not found: %w", apperr.ErrNotFound)
		}
		ts := e.targetsOf(a.ResourceID, a.Selector)
		if approve && a.Kind == KindAccess && len(ts) == 0 {
			if len(a.Selector) == 0 { // the resource was deleted: resource_id is NULL
				return fmt.Errorf("resource not found: %w", apperr.ErrNotFound)
			}
			return invalid("no resources match selector")
		}
		// owner and admin may deny any request but their own, even with no matching policy
		managerDeny := !approve && e.members[p.UserID].manager() && p.UserID != a.RequesterID
		if !managerDeny && !e.canDecide(a, p.UserID) {
			return denied("you cannot decide this request")
		}
		arg := dbq.DecideApprovalParams{Status: StatusDenied, DecidedBy: &p.UserID, Comment: comment, ID: id, OrgID: p.OrgID}
		action := "access.request.deny"
		if approve {
			arg.Status, action = StatusApproved, "access.request.approve"
		}
		if approve && a.Kind == KindAccess {
			t, _ := e.terms(a)
			if a.Duration, err = grantDuration(a.Duration, t, set.GrantMaxTTL, set.GrantDefaultTTL, false); err != nil {
				return err
			}
			arg.GrantSecs = new(a.Duration.Seconds())
		}
		if out, err = decide(ctx, st, arg); err != nil {
			return err
		}
		if err := audit(ctx, tx, p.OrgID, p.UserID, action, "approval", id, map[string]any{"kind": a.Kind, "comment": comment}); err != nil {
			return err
		}
		if a.Kind == KindCommand {
			if out.InstanceID == nil {
				return nil
			}
			return st.NotifyApprovalDecided(ctx, out.InstanceID.String()+":"+id.String())
		}
		if approve {
			if err := grant(ctx, st, tx, out, ts, a.Duration, &p.UserID); err != nil {
				return err
			}
		}
		return s.mail(ctx, tx, []member{e.members[a.RequesterID]}, "access_decision", map[string]string{
			"url": s.RequestURL(id), "resource": e.resourceText(a), "status": arg.Status, "comment": comment})
	})
	if err != nil {
		return Approval{}, err
	}
	return out, nil
}

// decide runs the guarded update; with no row it tells an expired request from a decided one.
func decide(ctx context.Context, st Store, arg dbq.DecideApprovalParams) (Approval, error) {
	row, err := st.DecideApproval(ctx, arg)
	if errors.Is(err, pgx.ErrNoRows) {
		state, serr := st.GetApprovalState(ctx, dbq.GetApprovalStateParams{ID: arg.ID, OrgID: arg.OrgID})
		switch {
		case errors.Is(serr, pgx.ErrNoRows):
			return Approval{}, fmt.Errorf("request not found: %w", apperr.ErrNotFound)
		case serr != nil:
			return Approval{}, serr
		case state.Status == StatusExpired || (state.Status == StatusPending && state.Overdue):
			return Approval{}, failed("request expired")
		}
		return Approval{}, failed("request is not pending")
	}
	if err != nil {
		return Approval{}, err
	}
	return toApproval(row)
}
