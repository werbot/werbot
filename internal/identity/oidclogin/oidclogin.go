// Package oidclogin signs members in with OpenID Connect identity providers
// (zitadel/oidc/v3 rp): the login routes under /api/oidc/, the IdP store and
// IdentityProviderService. Accounts are linked by verified email only.
package oidclogin

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// DefaultScopes are asked of an IdP stored without scopes.
var DefaultScopes = []string{"openid", "email", "profile", "groups"}

// Config holds the RP settings from the server configuration. HashKey and
// BlockKey sign and encrypt the login cookies (state, PKCE verifier, nonce,
// next): SHA-256("werbot-oidc-hash" ‖ WERBOT_SECRET_KEY) and
// SHA-256("werbot-oidc-block" ‖ WERBOT_SECRET_KEY). SecureCookies is false
// only for WERBOT_ENV=development, which also allows http:// issuers.
type Config struct {
	WebURL        string
	SecureCookies bool
	HashKey       []byte
	BlockKey      []byte
	HTTPClient    *http.Client // nil: a client with a 10 s timeout
}

// redirectURI is the callback the IdPs redirect to.
func (c Config) redirectURI() string { return c.WebURL + "/api/oidc/callback" }

// MappingRule matches Claim against Regex (anchored) and gives TeamID or Role.
type MappingRule struct {
	Claim  string     `json:"claim"`
	Regex  string     `json:"regex"`
	TeamID *uuid.UUID `json:"team_id,omitempty"`
	Role   string     `json:"role,omitempty"`
}

// IdP is an OIDC identity provider of an organization.
type IdP struct { //nolint:revive // name fixed by contract
	ID, OrgID                            uuid.UUID
	Name, Issuer, ClientID, ClientSecret string
	Scopes                               []string
	Mapping                              []MappingRule
	AllowSignup, Enabled                 bool
	CreatedAt                            time.Time
}

// Link is a member's identity at an IdP; Refresh is the opened refresh token.
type Link struct {
	IdPID, OrgID, UserID uuid.UUID //nolint:revive // name fixed by contract
	Subject, Refresh     string
}

// IdPStore is what the login routes and idp.recheck need of the store.
type IdPStore interface { //nolint:revive // name fixed by contract
	// Get returns the IdP id of any organization with its secret opened.
	Get(ctx context.Context, id uuid.UUID) (IdP, error)
	// SealRefresh seals an IdP refresh token for ExternalLogin.
	SealRefresh(refresh string) (ciphertext []byte, keyID string, err error)
	// StaleLinks returns links with a refresh token not checked for olderThan, oldest first.
	StaleLinks(ctx context.Context, olderThan time.Duration, limit int) ([]Link, error)
	// UserStaleLinks returns the links of userID in orgID with a refresh token not checked for olderThan.
	UserStaleLinks(ctx context.Context, orgID, userID uuid.UUID, olderThan time.Duration) ([]Link, error)
	// UpdateLink records a check at checkedAt; refresh nil keeps the token, "" drops it.
	UpdateLink(ctx context.Context, l Link, refresh *string, checkedAt time.Time) error
	// RevokeLink drops the refresh token the IdP refused and audits idp.recheck.revoke.
	RevokeLink(ctx context.Context, l Link) error
}
