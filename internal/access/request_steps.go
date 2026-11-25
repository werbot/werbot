package access

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/platform/apperr"
)

// createTx runs a new access request inside the transaction: policy lookup, then an
// instant approval (auto, break-glass) or a pending request with mail to approvers.
func (s *Service) createTx(ctx context.Context, st Store, tx pgx.Tx, a *Approval, in CreateRequestInput, set Settings) error {
	e, err := loadEvaluator(ctx, st, a.OrgID)
	if err != nil {
		return err
	}
	ts := e.targetsOf(a.ResourceID, a.Selector)
	if err := checkRequestTargets(*a, ts); err != nil {
		return err
	}
	t, err := requestTerms(e, *a)
	if err != nil {
		return err
	}
	if a.Duration, err = grantDuration(in.Duration, t, set.GrantMaxTTL, set.GrantDefaultTTL, a.BreakGlass); err != nil {
		return err
	}
	a.PolicyID = &t.PolicyID
	if a.BreakGlass || t.Auto {
		return s.createApproved(ctx, st, tx, e, a, ts)
	}
	return s.createPending(ctx, st, tx, e, a, t, set)
}

// checkRequestTargets refuses a request that covers no resource.
func checkRequestTargets(a Approval, ts []target) error {
	switch {
	case len(ts) == 0 && a.ResourceID != nil:
		return fmt.Errorf("resource not found: %w", apperr.ErrNotFound)
	case len(ts) == 0:
		return invalid("no resources match selector")
	}
	return nil
}

// requestTerms returns the policy terms of a, or refuses it when none apply or the reason is missing.
func requestTerms(e evaluator, a Approval) (terms, error) {
	t, ok := e.terms(a)
	switch {
	case !ok && a.BreakGlass:
		return terms{}, denied("break-glass is not allowed for this resource")
	case !ok:
		return terms{}, denied("no approval policy matches this request")
	case a.Reason == "" && (t.RequireReason || a.BreakGlass):
		return terms{}, invalid("reason is required")
	}
	return t, nil
}

// createPending stores a as pending, audits it and mails its approvers.
func (s *Service) createPending(ctx context.Context, st Store, tx pgx.Tx, e evaluator, a *Approval, t terms, set Settings) error {
	to := e.approvers(*a, t)
	if len(to) == 0 {
		return failed("no one can approve this request")
	}
	stored, err := insert(ctx, st, *a, StatusPending, set.RequestPendingTTL)
	if err != nil {
		return err
	}
	*a = stored
	if err := audit(ctx, tx, a.OrgID, a.RequesterID, "access.request.create", "approval", a.ID, createData(*a)); err != nil {
		return err
	}
	return s.mail(ctx, tx, to, "access_request", s.requestMail(e, *a, a.Duration))
}
