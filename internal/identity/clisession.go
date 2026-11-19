package identity

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/identity/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// CLISession is a live CLI sign-in (werbot login). Current marks the
// session of the caller's access token.
type CLISession struct {
	ID, UserID                                          uuid.UUID
	ClientID, Hostname                                  string
	IP                                                  netip.Addr
	CreatedAt, LastUsedAt, ExpiresAt, AbsoluteExpiresAt time.Time
	Current                                             bool
}

// isManager reports an owner or admin of the principal's organization.
func isManager(p principal.Principal) bool {
	return p.Role == principal.RoleOwner || p.Role == principal.RoleAdmin
}

// ListCLISessions returns the live CLI sessions of userID (nil: the caller)
// in the caller's organization, newest first. Another user's sessions need
// owner or admin.
func (s *Service) ListCLISessions(ctx context.Context, p principal.Principal, userID *uuid.UUID) ([]CLISession, error) {
	target := p.UserID
	if userID != nil && *userID != p.UserID {
		if !isManager(p) {
			return nil, apperr.ErrPermissionDenied
		}
		target = *userID
	}
	var out []CLISession
	err := dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		rows, err := newStore(tx).ListCLISessions(ctx, dbq.ListCLISessionsParams{OrgID: p.OrgID, UserID: target})
		for _, r := range rows {
			c := CLISession{
				ID: r.ID, UserID: r.UserID, ClientID: r.ClientID, Hostname: r.Hostname,
				CreatedAt: r.CreatedAt, LastUsedAt: r.LastUsedAt, ExpiresAt: r.ExpiresAt, AbsoluteExpiresAt: r.AbsoluteExpiresAt,
				Current: r.ID == p.CLISessionID,
			}
			if r.Ip != nil {
				c.IP = *r.Ip
			}
			out = append(out, c)
		}
		return err
	})
	return out, err
}

// RevokeCLISession ends session id: the caller's own, or any in the
// organization for an owner or admin. The next request with its access token
// and its refresh token are refused. A session that is not there, not the
// caller's or already revoked is ErrNotFound.
func (s *Service) RevokeCLISession(ctx context.Context, p principal.Principal, id uuid.UUID, reason string) error {
	return dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		n, err := newStore(tx).RevokeCLISession(ctx, dbq.RevokeCLISessionParams{
			Reason: reason, ID: id, OrgID: p.OrgID, Manager: isManager(p), UserID: p.UserID,
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("cli session not found: %w", apperr.ErrNotFound)
		}
		return auditlog.Write(ctx, tx, auditlog.Event{
			OrgID: &p.OrgID, ActorUserID: &p.UserID, ActorKind: "user", Action: "cli_session.revoke",
			TargetType: "cli_session", TargetID: id.String(), IP: clientip.From(ctx), Data: map[string]any{"reason": reason},
		})
	})
}
