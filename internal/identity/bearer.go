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
// statement checks that its cli_session is live and the user still a member.
// A bad header or token and a revoked, expired or missing session give
// ErrUnauthenticated; a store failure is returned as is.
func (s *Service) authenticateBearer(ctx context.Context, bearer BearerVerifier, header string) (principal.Principal, error) {
	scheme, raw, _ := strings.Cut(header, " ")
	if !strings.EqualFold(scheme, "Bearer") || raw == "" {
		return principal.Principal{}, fmt.Errorf("authorization scheme: %w", apperr.ErrUnauthenticated)
	}
	c, err := bearer.VerifyAccessToken(ctx, raw)
	if err != nil {
		return principal.Principal{}, fmt.Errorf("access token: %w", apperr.ErrUnauthenticated)
	}
	var role string
	err = dbx.ReadInOrg(s.pool, c.OrgID, c.UserID, func(rd *dbx.Read) error {
		var err error
		role, err = newStore(rd).GetCLISessionRole(ctx, dbq.GetCLISessionRoleParams{ID: c.CLISessionID, OrgID: c.OrgID, UserID: c.UserID})
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return principal.Principal{}, fmt.Errorf("cli session ended: %w", apperr.ErrUnauthenticated)
	}
	if err != nil {
		return principal.Principal{}, err
	}
	return principal.Principal{
		UserID: c.UserID, OrgID: c.OrgID, Role: role, AuthMethod: principal.AuthMethodOAuth, CLISessionID: c.CLISessionID,
	}, nil
}
