// Package identity owns users, memberships and cookie sessions: setup, login,
// logout and the middleware that turns a session cookie into a principal.
package identity

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/platform/apperr"
)

// Config holds identity settings from the server configuration.
type Config struct {
	WebURL        string // base of links in emails (config.WebURL)
	SecureCookies bool   // false only for WERBOT_ENV=development
}

// EmailQueue enqueues an email in the caller's transaction.
type EmailQueue interface {
	EnqueueTx(ctx context.Context, tx pgx.Tx, to, template string, data map[string]string) error
}

// User is a person who can sign in.
type User struct {
	ID                 uuid.UUID
	Email, Login, Name string
	CreatedAt          time.Time
}

// SessionInfo describes the current session's user, organization and role.
type SessionInfo struct {
	User    User
	OrgID   uuid.UUID
	OrgName string
	Role    string
}

// SetupInput is the first owner and organization created by Setup.
type SetupInput struct {
	Email, Login, Name, Password, OrganizationName string
}

// UserRef identifies an organization member for other domains.
type UserRef struct {
	UserID uuid.UUID
	Login  string
	Role   string
}

// errInvalidCredentials is the single answer for an unknown email, a wrong
// password and a user without membership.
var errInvalidCredentials = fmt.Errorf("invalid credentials: %w", apperr.ErrUnauthenticated)
