// Package oauth is the OAuth 2.0 authorization server of werbot server for the
// CLI (zitadel/oidc op). Its state lives only in Postgres, so any replica
// serves any step of a flow.
package oauth

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const (
	// ClientCLI is the only client: the public, native werbot CLI.
	ClientCLI = "werbot-cli"
	// AccessTokenTTL is the lifetime of access and ID tokens.
	AccessTokenTTL = 10 * time.Minute
	// Audience is the aud of every access token.
	Audience = "werbot-api"
)

// Config configures the authorization server.
type Config struct {
	Issuer string // WERBOT_PUBLIC_URL: iss of every token; an http issuer is allowed (insecure, see docs/security.md)
	WebURL string // WERBOT_WEB_URL: consent and device pages
}

// Users reports a user's role in an organization (*identity.Service).
type Users interface {
	Membership(ctx context.Context, orgID, userID uuid.UUID) (role string, err error)
}
