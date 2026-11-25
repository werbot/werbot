package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/identity/dbq"
	"github.com/werbot/werbot/internal/identity/oauth"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// BearerVerifier checks an access token without the database (*oauth.Provider).
type BearerVerifier interface {
	VerifyAccessToken(ctx context.Context, raw string) (oauth.Claims, error)
}

// authenticateBearer resolves an Authorization header to the principal of a
// CLI access token: the token is verified without the database, then one
// statement checks that its cli_session is live and the user still a member
// and reads the user's MFA state (FreshAt: the newest second factor
// verified for this cli_session).
// An agent run token is its sponsor's principal with AgentID and RunID; a
// session bound to an agent (an agent profile) admits only that agent's tokens.
// A bad header or token and a revoked, expired or missing session give
// ErrUnauthenticated; a store failure is returned as is.
func (s *Service) authenticateBearer(ctx context.Context, bearer BearerVerifier, header string) (principal.Principal, mfaState, error) {
	scheme, raw, _ := strings.Cut(header, " ")
	if !strings.EqualFold(scheme, "Bearer") || raw == "" {
		return principal.Principal{}, mfaState{}, fmt.Errorf("authorization scheme: %w", apperr.ErrUnauthenticated)
	}
	c, err := bearer.VerifyAccessToken(ctx, raw)
	if err != nil {
		return principal.Principal{}, mfaState{}, fmt.Errorf("access token: %w", apperr.ErrUnauthenticated)
	}
	var row dbq.GetCLISessionRoleRow
	err = dbx.ReadInOrg(s.pool, c.OrgID, c.UserID, func(rd *dbx.Read) error {
		var err error
		row, err = newStore(rd).GetCLISessionRole(ctx, dbq.GetCLISessionRoleParams{ID: c.CLISessionID, OrgID: c.OrgID, UserID: c.UserID})
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return principal.Principal{}, mfaState{}, fmt.Errorf("cli session ended: %w", apperr.ErrUnauthenticated)
	}
	if err != nil {
		return principal.Principal{}, mfaState{}, err
	}
	if row.AgentID != nil && (c.AgentID == nil || *c.AgentID != *row.AgentID) {
		return principal.Principal{}, mfaState{}, fmt.Errorf("agent profile token: %w", apperr.ErrUnauthenticated)
	}
	return principal.Principal{
		UserID: c.UserID, OrgID: c.OrgID, Role: row.Role, AuthMethod: principal.AuthMethodOAuth, CLISessionID: c.CLISessionID,
		AgentID: c.AgentID, RunID: c.RunID,
	}, mfaState{RequireMFA: row.RequireMfa, HasFactor: row.HasFactor, FreshAt: row.MfaAt}, nil
}
