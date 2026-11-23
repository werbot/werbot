package oauth

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
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
// jti (the cli_session) and werbot_org; an agent token also act.sub
// agent:<uuid> and UUID werbot_run, never one without the other. An ID token
// has no jti and no werbot_org, so it never passes. Any failure is
// ErrUnauthenticated.
func (p *Provider) VerifyAccessToken(ctx context.Context, raw string) (Claims, error) {
	c, err := op.VerifyAccessToken[*oidc.AccessTokenClaims](ctx, raw, p.verifier)
	if err != nil || !slices.Contains(c.Audience, Audience) {
		return Claims{}, fmt.Errorf("access token: %w", apperr.ErrUnauthenticated)
	}
	org, _ := c.Claims[claimOrg].(string)
	user, err1 := parseID(c.Subject)
	sess, err2 := parseID(c.JWTID)
	orgID, err3 := parseID(org)
	agent, run, err4 := agentClaims(c)
	if errors.Join(err1, err2, err3, err4) != nil {
		return Claims{}, fmt.Errorf("access token claims: %w", apperr.ErrUnauthenticated)
	}
	return Claims{UserID: user, OrgID: orgID, CLISessionID: sess, AgentID: agent, RunID: run, Expiry: c.GetExpiration()}, nil
}

// agentClaims reads the agent of act.sub (agent:<uuid>) and werbot_run:
// both or neither.
func agentClaims(c *oidc.AccessTokenClaims) (agent, run *uuid.UUID, err error) {
	runClaim, hasRun := c.Claims[claimRun]
	if c.Actor == nil && !hasRun {
		return nil, nil, nil
	}
	if c.Actor == nil || !hasRun {
		return nil, nil, errors.New("agent claims need each other")
	}
	name, isAgent := strings.CutPrefix(c.Actor.Subject, AgentScopePrefix)
	runID, _ := runClaim.(string)
	a, err1 := parseID(name)
	r, err2 := parseID(runID)
	if !isAgent || errors.Join(err1, err2) != nil {
		return nil, nil, errors.New("invalid agent claims")
	}
	return &a, &r, nil
}

// parseID parses a non-nil UUID.
func parseID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err == nil && id == uuid.Nil {
		err = errors.New("nil uuid")
	}
	return id, err
}
