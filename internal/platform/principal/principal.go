// Package principal carries the authenticated subject through the request context.
package principal

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/platform/apperr"
)

// Organization roles.
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// Principal is the authenticated user acting within one organization.
type Principal struct {
	UserID, OrgID uuid.UUID
	Role          string
	SessionID     uuid.UUID
}

type ctxKey struct{}

// With returns ctx carrying p.
func With(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// From returns the principal stored in ctx.
func From(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}

// Require returns the principal or apperr.ErrUnauthenticated.
func Require(ctx context.Context) (Principal, error) {
	p, ok := From(ctx)
	if !ok {
		return Principal{}, apperr.ErrUnauthenticated
	}
	return p, nil
}

// RequireRole is Require plus a role check: apperr.ErrPermissionDenied when p.Role is not in roles.
func RequireRole(ctx context.Context, roles ...string) (Principal, error) {
	p, err := Require(ctx)
	if err != nil {
		return Principal{}, err
	}
	if !slices.Contains(roles, p.Role) {
		return Principal{}, apperr.ErrPermissionDenied
	}
	return p, nil
}
