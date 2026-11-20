package access

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/access/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

const maxReason = 1024

func denied(msg string) error { return fmt.Errorf("%s: %w", msg, apperr.ErrPermissionDenied) }
func failed(msg string) error { return fmt.Errorf("%s: %w", msg, apperr.ErrFailedPrecondition) }

// text trims s and refuses more than maxReason characters.
func text(name, s string) (string, error) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > maxReason {
		return "", invalid("%s must have at most %d characters", name, maxReason)
	}
	return s, nil
}

func (in CreateRequestInput) validate() error {
	if (in.ResourceID != nil) == (len(in.Selector) > 0) {
		return invalid("set exactly one of resource and label selector")
	}
	if in.ResourceID == nil {
		return Selector{Labels: in.Selector}.validate()
	}
	return nil
}

func toApproval(r dbq.Approval) (Approval, error) {
	a := Approval{ID: r.ID, OrgID: r.OrgID, RequesterID: r.RequesterUserID, Kind: ApprovalKind(r.Kind), AgentRunID: r.AgentRunID,
		ResourceID: r.ResourceID, PolicyID: r.PolicyID, DecidedBy: r.DecidedBy, InstanceID: r.GatewayInstanceID,
		Duration:      time.Duration(r.Duration.Microseconds)*time.Microsecond + time.Duration(r.Duration.Days)*24*time.Hour,
		CommandSHA256: r.CommandSha256, Classes: r.Classes, Reason: r.Reason, Status: r.Status, Comment: r.Comment,
		BreakGlass: r.BreakGlass, ExpiresAt: r.ExpiresAt, CreatedAt: r.CreatedAt, DecidedAt: r.DecidedAt}
	if r.Command != nil {
		a.Command = *r.Command
	}
	if r.LabelSelector != nil {
		if err := json.Unmarshal(r.LabelSelector, &a.Selector); err != nil {
			return Approval{}, fmt.Errorf("access: selector of approval %s: %w", r.ID, err)
		}
	}
	return a, nil
}

// insert stores a with status, expiring after ttl, and returns the stored row.
func insert(ctx context.Context, st Store, a Approval, status string, ttl time.Duration) (Approval, error) {
	arg := dbq.CreateApprovalParams{OrgID: a.OrgID, Kind: string(a.Kind), RequesterUserID: a.RequesterID, AgentRunID: a.AgentRunID,
		ResourceID: a.ResourceID, CommandSha256: a.CommandSHA256, Classes: a.Classes, Reason: a.Reason, PolicyID: a.PolicyID,
		BreakGlass: a.BreakGlass, Status: status, ExpiresSecs: ttl.Seconds(), GatewayInstanceID: a.InstanceID}
	if a.Kind == KindAccess {
		arg.DurationSecs = new(a.Duration.Seconds())
	} else {
		arg.Command = &a.Command
	}
	if len(a.Selector) > 0 {
		var err error
		if arg.LabelSelector, err = json.Marshal(a.Selector); err != nil {
			return Approval{}, err
		}
	}
	row, err := st.CreateApproval(ctx, arg)
	if err != nil {
		return Approval{}, err
	}
	return toApproval(row)
}

// targetIDs returns the resource ids of ts.
func targetIDs(ts []target) []uuid.UUID {
	ids := make([]uuid.UUID, len(ts))
	for i, t := range ts {
		ids[i] = t.ID
	}
	return ids
}

// grant gives the requester of a every resource of ts for d and audits each grant.
func grant(ctx context.Context, st Store, tx pgx.Tx, a Approval, ts []target, d time.Duration, actor *uuid.UUID) error {
	var createdBy *uuid.UUID
	if a.BreakGlass {
		createdBy = &a.RequesterID
	}
	rows, err := st.CreateGrants(ctx, dbq.CreateGrantsParams{OrgID: a.OrgID, ApprovalID: a.ID, UserID: a.RequesterID,
		DurationSecs: d.Seconds(), BreakGlass: a.BreakGlass, CreatedBy: createdBy, ResourceIds: targetIDs(ts)})
	if err != nil {
		return err
	}
	kind := "user"
	if actor == nil {
		kind = "system"
	}
	for _, g := range rows {
		if err := auditAs(ctx, tx, a.OrgID, kind, actor, "access.grant.create", "access_grant", g.ID, map[string]any{
			"approval_id": a.ID, "user_id": a.RequesterID, "resource_id": g.ResourceID, "expires_at": g.ExpiresAt, "break_glass": a.BreakGlass,
		}); err != nil {
			return err
		}
	}
	return nil
}

// CreateRequest asks for access for p: auto and break-glass requests are approved at once, others wait for an approver.
func (s *Service) CreateRequest(ctx context.Context, p principal.Principal, in CreateRequestInput) (Approval, error) {
	if err := in.validate(); err != nil {
		return Approval{}, err
	}
	reason, err := text("reason", in.Reason)
	if err != nil {
		return Approval{}, err
	}
	set, err := s.settings.AccessSettings(ctx, p.OrgID)
	if err != nil {
		return Approval{}, err
	}
	a := Approval{OrgID: p.OrgID, RequesterID: p.UserID, Kind: KindAccess, ResourceID: in.ResourceID, Selector: in.Selector,
		Reason: reason, BreakGlass: in.BreakGlass}
	err = s.tx(ctx, p.OrgID, p.UserID, func(st Store, tx pgx.Tx) error {
		e, err := loadEvaluator(ctx, st, p.OrgID)
		if err != nil {
			return err
		}
		ts := e.targetsOf(a.ResourceID, a.Selector)
		switch {
		case len(ts) == 0 && a.ResourceID != nil:
			return fmt.Errorf("resource not found: %w", apperr.ErrNotFound)
		case len(ts) == 0:
			return invalid("no resources match selector")
		}
		t, ok := e.terms(a)
		switch {
		case !ok && a.BreakGlass:
			return denied("break-glass is not allowed for this resource")
		case !ok:
			return denied("no approval policy matches this request")
		case reason == "" && (t.RequireReason || a.BreakGlass):
			return invalid("reason is required")
		}
		if a.Duration, err = grantDuration(in.Duration, t, set.GrantMaxTTL, set.GrantDefaultTTL, a.BreakGlass); err != nil {
			return err
		}
		a.PolicyID = &t.PolicyID
		if a.BreakGlass || t.Auto {
			return s.createApproved(ctx, st, tx, e, &a, ts)
		}
		to := e.approvers(a, t)
		if len(to) == 0 {
			return failed("no one can approve this request")
		}
		if a, err = insert(ctx, st, a, StatusPending, set.RequestPendingTTL); err != nil {
			return err
		}
		if err := audit(ctx, tx, p.OrgID, p.UserID, "access.request.create", "approval", a.ID, createData(a)); err != nil {
			return err
		}
		return s.mail(ctx, tx, to, "access_request", s.requestMail(e, a, a.Duration))
	})
	if err != nil {
		return Approval{}, err
	}
	return a, nil
}

// createApproved stores an auto or break-glass request as approved and grants it.
func (s *Service) createApproved(ctx context.Context, st Store, tx pgx.Tx, e evaluator, a *Approval, ts []target) error {
	stored, err := insert(ctx, st, *a, StatusApproved, a.Duration)
	if err != nil {
		return err
	}
	*a = stored
	if err := audit(ctx, tx, a.OrgID, a.RequesterID, "access.request.create", "approval", a.ID, createData(*a)); err != nil {
		return err
	}
	if !a.BreakGlass {
		if err := auditAs(ctx, tx, a.OrgID, "system", nil, "access.request.approve", "approval", a.ID, map[string]any{"auto": true}); err != nil {
			return err
		}
		return grant(ctx, st, tx, *a, ts, a.Duration, nil)
	}
	if err := audit(ctx, tx, a.OrgID, a.RequesterID, "access.break_glass", "approval", a.ID, map[string]any{
		"approval_id": a.ID, "resource_ids": targetIDs(ts), "duration": formatDuration(a.Duration), "reason": a.Reason,
	}); err != nil {
		return err
	}
	if err := grant(ctx, st, tx, *a, ts, a.Duration, &a.RequesterID); err != nil {
		return err
	}
	return s.mail(ctx, tx, e.membersWhere(func(m member) bool { return isManager(m.Role) }), "break_glass", map[string]string{
		"url": s.RequestURL(a.ID), "requester": e.members[a.RequesterID].Login, "resource": e.resourceText(*a), "reason": a.Reason})
}

// createData is the audit data of a new access request.
func createData(a Approval) map[string]any {
	d := map[string]any{"kind": a.Kind, "duration": formatDuration(a.Duration), "break_glass": a.BreakGlass}
	if a.ResourceID != nil {
		d["resource_id"] = a.ResourceID
	} else {
		d["label_selector"] = a.Selector
	}
	return d
}

// Cancel withdraws a pending access request of p; a request of someone else is not found.
func (s *Service) Cancel(ctx context.Context, p principal.Principal, id uuid.UUID) error {
	return s.tx(ctx, p.OrgID, p.UserID, func(st Store, tx pgx.Tx) error {
		_, err := st.CancelApproval(ctx, dbq.CancelApprovalParams{ID: id, OrgID: p.OrgID, RequesterUserID: p.UserID})
		if errors.Is(err, pgx.ErrNoRows) {
			row, gerr := st.GetApproval(ctx, dbq.GetApprovalParams{ID: id, OrgID: p.OrgID})
			switch {
			case errors.Is(gerr, pgx.ErrNoRows) || (gerr == nil && (row.RequesterUserID != p.UserID || row.Kind != string(KindAccess))):
				return fmt.Errorf("request not found: %w", apperr.ErrNotFound)
			case gerr != nil:
				return gerr
			}
			return failed("request is not pending")
		}
		if err != nil {
			return err
		}
		return audit(ctx, tx, p.OrgID, p.UserID, "access.request.cancel", "approval", id, nil)
	})
}
