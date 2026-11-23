// Package principal carries the authenticated subject through the request context.
package principal

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/platform/apperr"
)

// Organization roles.
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// How the principal authenticated.
const (
	AuthMethodCookie = "cookie"
	AuthMethodOAuth  = "oauth"
)

// Principal is the authenticated user acting within one organization.
// SessionID is the cookie session, CLISessionID the CLI session of a Bearer
// token; the other is the zero UUID. AgentID and RunID are set together, only
// for an agent run token: UserID is then the agent's sponsor. MFAAt is when
// the cookie session last passed a second factor (nil: never; always nil for
// a Bearer token).
type Principal struct {
	UserID, OrgID  uuid.UUID
	Role           string
	SessionID      uuid.UUID
	AuthMethod     string
	CLISessionID   uuid.UUID
	AgentID, RunID *uuid.UUID
	MFAAt          *time.Time
}

// IsAgent reports whether p is an agent run token.
func (p Principal) IsAgent() bool { return p.AgentID != nil || p.RunID != nil }

// Manager reports whether p is an organization owner or admin.
func (p Principal) Manager() bool { return p.Role == RoleOwner || p.Role == RoleAdmin }

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
