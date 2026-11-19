package oauth

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/identity/oauth/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// ClientInfo is what the consent page shows about a pending sign-in.
type ClientInfo struct {
	ClientID, Hostname string
	IP                 netip.Addr
	Scopes             []string
}

var errNoRequest = fmt.Errorf("auth request: %w", apperr.ErrNotFound)

// requireCookie: only a browser session consents; a CLI token never
// approves another CLI.
func requireCookie(p principal.Principal) error {
	if p.AuthMethod != principal.AuthMethodCookie {
		return fmt.Errorf("consent needs a browser session: %w", apperr.ErrPermissionDenied)
	}
	return nil
}

// GetAuthRequest returns the pending (unapproved, live) request id.
func (p *Provider) GetAuthRequest(ctx context.Context, pr principal.Principal, id uuid.UUID) (ClientInfo, error) {
	if err := requireCookie(pr); err != nil {
		return ClientInfo{}, err
	}
	var info ClientInfo
	err := dbx.System(ctx, p.pool, dbx.ReasonLogin, func(tx pgx.Tx) error {
		r, err := dbq.New(tx).GetPendingAuthRequest(ctx, id)
		if err != nil {
			return err
		}
		info = ClientInfo{ClientID: r.ClientID, Hostname: r.ClientHostname, Scopes: r.Scopes}
		if r.ClientIp != nil {
			info.IP = *r.ClientIp
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ClientInfo{}, errNoRequest
	}
	return info, err
}

// ApproveAuthRequest binds the pending request id to the caller and returns
// the URL that hands the CLI its code (op's callback, through WebURL).
func (p *Provider) ApproveAuthRequest(ctx context.Context, pr principal.Principal, id uuid.UUID) (string, error) {
	if err := requireCookie(pr); err != nil {
		return "", err
	}
	if _, err := p.users.Membership(ctx, pr.OrgID, pr.UserID); err != nil {
		return "", err
	}
	return p.decide(ctx, id, func(q *dbq.Queries) (int64, error) {
		return q.ApproveAuthRequest(ctx, dbq.ApproveAuthRequestParams{ID: id, UserID: &pr.UserID, OrgID: &pr.OrgID})
	})
}

// DenyAuthRequest ends the pending request id; its callback URL hands the
// CLI interaction_required at once instead of a wait until expiry.
func (p *Provider) DenyAuthRequest(ctx context.Context, pr principal.Principal, id uuid.UUID) (string, error) {
	if err := requireCookie(pr); err != nil {
		return "", err
	}
	return p.decide(ctx, id, func(q *dbq.Queries) (int64, error) { return q.DenyAuthRequest(ctx, id) })
}

// decide runs the update of a pending request; no row is ErrNotFound.
func (p *Provider) decide(ctx context.Context, id uuid.UUID, update func(*dbq.Queries) (int64, error)) (string, error) {
	err := dbx.System(ctx, p.pool, dbx.ReasonLogin, func(tx pgx.Tx) error {
		n, err := update(dbq.New(tx))
		if err == nil && n == 0 {
			return errNoRequest
		}
		return err
	})
	if err != nil {
		return "", err
	}
	return p.webURL + "/oauth/authorize/callback?id=" + id.String(), nil
}
