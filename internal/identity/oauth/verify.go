package oauth

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/werbot/werbot/internal/platform/apperr"
)

// Claims are the subject of a verified access token.
type Claims struct {
	UserID, OrgID, CLISessionID uuid.UUID
	AgentID, RunID              *uuid.UUID // agent tokens (stage 12); nil for CLI tokens
	Expiry                      time.Time
}

// VerifyAccessToken checks an access token without the database: ES256 by
// the signing key, this issuer, not expired, aud ∋ werbot-api, and UUID sub,
// jti (the cli_session) and werbot_org. An ID token has no jti and no
// werbot_org, so it never passes. Any failure is ErrUnauthenticated.
func (p *Provider) VerifyAccessToken(ctx context.Context, raw string) (Claims, error) {
	c, err := op.VerifyAccessToken[*oidc.AccessTokenClaims](ctx, raw, p.verifier)
	if err != nil || !slices.Contains(c.Audience, Audience) {
		return Claims{}, fmt.Errorf("access token: %w", apperr.ErrUnauthenticated)
	}
	org, _ := c.Claims[claimOrg].(string)
	user, err1 := parseID(c.Subject)
	sess, err2 := parseID(c.JWTID)
	orgID, err3 := parseID(org)
	if err1 != nil || err2 != nil || err3 != nil {
		return Claims{}, fmt.Errorf("access token claims: %w", apperr.ErrUnauthenticated)
	}
	return Claims{UserID: user, OrgID: orgID, CLISessionID: sess, Expiry: c.GetExpiration()}, nil
}

// parseID parses a non-nil UUID.
func parseID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err == nil && id == uuid.Nil {
		err = errors.New("nil uuid")
	}
	return id, err
}
