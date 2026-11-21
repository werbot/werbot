package access

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	err = s.tx(ctx, p.OrgID, p.UserID, func(st Store, tx pgx.Tx) (err error) {
		out, err = s.decideTx(ctx, st, tx, p, id, approve, comment, set)
		return err
	})
	if err != nil {
		return Approval{}, err
	}
	return out, nil
}

// decideTx runs the decision inside the transaction: guarded update, audit, then grants, mail or NOTIFY.
func (s *Service) decideTx(ctx context.Context, st Store, tx pgx.Tx, p principal.Principal, id uuid.UUID, approve bool, comment string, set Settings) (Approval, error) {
	a, e, ts, err := loadDecidable(ctx, st, p, id, approve)
	if err != nil {
		return Approval{}, err
	}
	arg, action := decisionArg(p, id, approve, comment)
	if approve && a.Kind == KindAccess {
		if a.Duration, err = approvedDuration(a, e, set); err != nil {
			return Approval{}, err
		}
		arg.GrantSecs = new(a.Duration.Seconds())
	}
	out, err := decide(ctx, st, arg)
	if err != nil {
		return Approval{}, err
	}
	if err := audit(ctx, tx, p.OrgID, p.UserID, action, "approval", id, map[string]any{"kind": a.Kind, "comment": comment}); err != nil {
		return Approval{}, err
	}
	return out, s.announce(ctx, st, tx, e, a, out, ts, approve, comment)
}

// loadDecidable loads the approval and refuses a caller who may not see or decide it.
func loadDecidable(ctx context.Context, st Store, p principal.Principal, id uuid.UUID, approve bool) (Approval, evaluator, []target, error) {
	row, err := st.GetApproval(ctx, dbq.GetApprovalParams{ID: id, OrgID: p.OrgID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Approval{}, evaluator{}, nil, fmt.Errorf("request not found: %w", apperr.ErrNotFound)
	}
	if err != nil {
		return Approval{}, evaluator{}, nil, err
	}
	a, err := toApproval(row)
	if err != nil {
		return Approval{}, evaluator{}, nil, err
	}
	e, err := loadEvaluator(ctx, st, p.OrgID)
	if err != nil {
		return Approval{}, evaluator{}, nil, err
	}
	if !e.visible(a, p.UserID) {
		return Approval{}, evaluator{}, nil, fmt.Errorf("request not found: %w", apperr.ErrNotFound)
	}
	ts := e.targetsOf(a.ResourceID, a.Selector)
	if err := checkApproveTargets(a, ts, approve); err != nil {
		return Approval{}, evaluator{}, nil, err
	}
	if !mayDecide(e, a, p.UserID, approve) {
		return Approval{}, evaluator{}, nil, denied("you cannot decide this request")
	}
	return a, e, ts, nil
}

// checkApproveTargets refuses to approve an access request that covers no resource.
func checkApproveTargets(a Approval, ts []target, approve bool) error {
	if !approve || a.Kind != KindAccess || len(ts) > 0 {
		return nil
	}
	if len(a.Selector) == 0 { // the resource was deleted: resource_id is NULL
		return fmt.Errorf("resource not found: %w", apperr.ErrNotFound)
	}
	return invalid("no resources match selector")
}

// mayDecide tells whether uid may decide a. Owner and admin may deny any request
// but their own, even with no matching policy.
func mayDecide(e evaluator, a Approval, uid uuid.UUID, approve bool) bool {
	managerDeny := !approve && isManager(e.members[uid].Role) && uid != a.RequesterID
	return managerDeny || e.canDecide(a, uid)
}

// decisionArg returns the guarded update arguments and the audit action of a decision.
func decisionArg(p principal.Principal, id uuid.UUID, approve bool, comment string) (dbq.DecideApprovalParams, string) {
	arg := dbq.DecideApprovalParams{Status: StatusDenied, DecidedBy: &p.UserID, Comment: comment, ID: id, OrgID: p.OrgID}
	if approve {
		arg.Status = StatusApproved
		return arg, "access.request.approve"
	}
	return arg, "access.request.deny"
}

// approvedDuration is the grant duration of an approved access request under its terms and the org limits.
func approvedDuration(a Approval, e evaluator, set Settings) (time.Duration, error) {
	t, _ := e.terms(a)
	return grantDuration(a.Duration, t, set.GrantMaxTTL, set.GrantDefaultTTL, false)
}

// announce tells the outcome of a decision: NOTIFY for a command, grants and mail for an access request.
func (s *Service) announce(ctx context.Context, st Store, tx pgx.Tx, e evaluator, a, out Approval, ts []target, approve bool, comment string) error {
	if a.Kind == KindCommand {
		if out.InstanceID == nil {
			return nil
		}
		return st.NotifyApprovalDecided(ctx, out.InstanceID.String()+":"+out.ID.String())
	}
	if approve {
		if err := grant(ctx, st, tx, out, ts, a.Duration, out.DecidedBy); err != nil {
			return err
		}
	}
	return s.mail(ctx, tx, []member{e.members[a.RequesterID]}, "access_decision", map[string]string{
		"url": s.RequestURL(out.ID), "resource": e.resourceText(a), "status": out.Status, "comment": comment})
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
