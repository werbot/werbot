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

// GrantFilter narrows ListGrants; nil ids match every grant, Active keeps unrevoked, unexpired ones.
type GrantFilter struct {
	UserID, ResourceID *uuid.UUID
	Active             bool
}

// GrantView is a grant with its user, resource and history.
type GrantView struct {
	Grant
	UserLogin, ResourceName string
	RevokedAt               *time.Time
	CreatedBy               *uuid.UUID
	CreatedAt               time.Time
}

// ActiveGrants returns the unrevoked, unexpired grants of orgID by id, for the policy snapshot.
func (s *Service) ActiveGrants(ctx context.Context, orgID uuid.UUID) ([]Grant, error) {
	var out []Grant
	err := s.tx(ctx, orgID, uuid.Nil, func(st Store, _ pgx.Tx) error {
		rows, err := st.ActiveGrants(ctx, orgID)
		for _, r := range rows {
			out = append(out, Grant{ID: r.ID, OrgID: r.OrgID, UserID: r.UserID, ResourceID: r.ResourceID, ApprovalID: r.ApprovalID,
				ExpiresAt: r.ExpiresAt, BreakGlass: r.BreakGlass, NeedsReview: r.NeedsReview})
		}
		return err
	})
	return out, err
}

// ListGrants returns the grants f matches, newest first: owner and admin see
// the organization's, a member only own ones.
func (s *Service) ListGrants(ctx context.Context, p principal.Principal, f GrantFilter) ([]GrantView, error) {
	if !isManager(p.Role) {
		if f.UserID != nil && *f.UserID != p.UserID {
			return nil, denied("members list only their own grants")
		}
		f.UserID = &p.UserID
	}
	var out []GrantView
	err := s.tx(ctx, p.OrgID, p.UserID, func(st Store, _ pgx.Tx) error {
		rows, err := st.ListGrants(ctx, dbq.ListGrantsParams{OrgID: p.OrgID, UserID: f.UserID, ResourceID: f.ResourceID, Active: f.Active})
		for _, r := range rows {
			out = append(out, GrantView{Grant: Grant{ID: r.ID, OrgID: r.OrgID, UserID: r.UserID, ResourceID: r.ResourceID,
				ApprovalID: r.ApprovalID, ExpiresAt: r.ExpiresAt, BreakGlass: r.BreakGlass, NeedsReview: r.NeedsReview},
				UserLogin: r.UserLogin, ResourceName: r.ResourceName, RevokedAt: r.RevokedAt, CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt})
		}
		return err
	})
	return out, err
}

// grantForUpdate reads grant id of orgID and holds its row until the transaction ends.
func grantForUpdate(ctx context.Context, st Store, orgID, id uuid.UUID) (dbq.GrantForUpdateRow, error) {
	g, err := st.GrantForUpdate(ctx, dbq.GrantForUpdateParams{ID: id, OrgID: orgID})
	if errors.Is(err, pgx.ErrNoRows) {
		return g, fmt.Errorf("grant not found: %w", apperr.ErrNotFound)
	}
	return g, err
}

// RevokeGrant ends grant id now: owner and admin revoke any grant, a member
// only own ones. A revoked grant stays revoked: nil, no second event.
func (s *Service) RevokeGrant(ctx context.Context, p principal.Principal, id uuid.UUID, reason string) error {
	reason, err := text("reason", reason)
	if err != nil {
		return err
	}
	return s.tx(ctx, p.OrgID, p.UserID, func(st Store, tx pgx.Tx) error {
		g, err := grantForUpdate(ctx, st, p.OrgID, id)
		switch {
		case err != nil:
			return err
		case !isManager(p.Role) && g.UserID != p.UserID:
			return denied("members revoke only their own grants")
		case g.Revoked:
			return nil
		}
		if err := st.RevokeGrant(ctx, dbq.RevokeGrantParams{ID: id, OrgID: p.OrgID}); err != nil {
			return err
		}
		return audit(ctx, tx, p.OrgID, p.UserID, "access.grant.revoke", "access_grant", id,
			map[string]any{"user_id": g.UserID, "resource_id": g.ResourceID, "reason": reason})
	})
}

// ReviewBreakGlass marks a break-glass grant reviewed: by an owner or admin
// who is not the grant's user, once.
func (s *Service) ReviewBreakGlass(ctx context.Context, p principal.Principal, grantID uuid.UUID, comment string) error {
	if !isManager(p.Role) {
		return denied("only owner and admin review break-glass")
	}
	comment, err := text("comment", comment)
	if err != nil {
		return err
	}
	return s.tx(ctx, p.OrgID, p.UserID, func(st Store, tx pgx.Tx) error {
		g, err := grantForUpdate(ctx, st, p.OrgID, grantID)
		switch {
		case err != nil:
			return err
		case g.UserID == p.UserID:
			return denied("you cannot review your own break-glass")
		case !g.BreakGlass || !g.NeedsReview:
			return failed("grant needs no review")
		}
		if err := st.ReviewGrant(ctx, dbq.ReviewGrantParams{ID: grantID, OrgID: p.OrgID}); err != nil {
			return err
		}
		return audit(ctx, tx, p.OrgID, p.UserID, "access.break_glass.review", "access_grant", grantID,
			map[string]any{"user_id": g.UserID, "resource_id": g.ResourceID, "comment": comment})
	})
}
