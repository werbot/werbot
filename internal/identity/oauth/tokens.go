package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/werbot/werbot/internal/identity/oauth/dbq"
	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/dbx"
)

// claimOrg is the private claim of the access token that names its organization.
const claimOrg = "werbot_org"

// newRefreshToken returns a refresh token (wrt_ + 32 random bytes) and its hash.
func newRefreshToken() (raw string, hash []byte) {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // never fails (crypto/rand)
	raw = "wrt_" + base64.RawURLEncoding.EncodeToString(b)
	return raw, hashSecret(raw)
}

// CreateAccessToken implements op.Storage: every grant of werbot-cli comes
// with a refresh token, so a token without one is a request without offline_access.
func (*storage) CreateAccessToken(context.Context, op.TokenRequest) (string, time.Time, error) {
	return "", time.Time{}, oidc.ErrInvalidScope().WithDescription("offline_access required")
}

// CreateAccessAndRefreshTokens implements op.Storage. The access token id
// (jti) is the cli_session id.
func (s *storage) CreateAccessAndRefreshTokens(ctx context.Context, request op.TokenRequest, _ string) (string, string, time.Time, error) {
	switch req := request.(type) {
	case *authRequest:
		// Of concurrent exchanges of one code exactly one deletes the row.
		return s.openSession(ctx, "loopback", func(q *dbq.Queries) (dbq.ConsumeAuthRequestRow, error) {
			return q.ConsumeAuthRequest(ctx, dbq.ConsumeAuthRequestParams{ID: req.ID, CodeHash: req.codeHash})
		})
	case *op.DeviceAuthorizationState:
		return s.exchangeDevice(ctx)
	default:
		return "", "", time.Time{}, errNoFlow
	}
}

// openSession consumes an approved grant and opens its cli_session, audited
// as cli.login of flow, in one transaction; a grant already consumed (or
// ended) is invalid_grant.
func (s *storage) openSession(ctx context.Context, flow string, consume func(*dbq.Queries) (dbq.ConsumeAuthRequestRow, error)) (string, string, time.Time, error) {
	raw, hash := newRefreshToken()
	var id uuid.UUID
	err := dbx.System(ctx, s.pool, dbx.ReasonLogin, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		row, err := consume(q)
		if errors.Is(err, pgx.ErrNoRows) {
			return oidc.ErrInvalidGrant().WithDescription("invalid code")
		}
		if err != nil {
			return err
		}
		id, err = q.CreateCLISession(ctx, dbq.CreateCLISessionParams{
			OrgID: *row.OrgID, UserID: *row.UserID, ClientID: row.ClientID, Hostname: row.ClientHostname,
			Ip: row.ClientIp, RefreshHash: hash,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("organization %s has no settings", *row.OrgID)
		}
		if err != nil {
			return err
		}
		var ip netip.Addr
		if row.ClientIp != nil {
			ip = *row.ClientIp
		}
		return auditlog.Write(ctx, tx, auditlog.Event{
			OrgID: row.OrgID, ActorUserID: row.UserID, ActorKind: "user", Action: "cli.login",
			TargetType: "cli_session", TargetID: id.String(), IP: ip,
			Data: map[string]any{"client_id": row.ClientID, "hostname": row.ClientHostname, "flow": flow},
		})
	})
	if err != nil {
		return "", "", time.Time{}, err
	}
	return id.String(), raw, time.Now().Add(AccessTokenTTL), nil
}

// GetPrivateClaimsFromRequest implements op.CanGetPrivateClaimsFromRequest:
// the access token names its organization. ID tokens never get it.
func (*storage) GetPrivateClaimsFromRequest(ctx context.Context, request op.TokenRequest, _ []string) (map[string]any, error) {
	var org uuid.UUID
	switch req := request.(type) {
	case *authRequest:
		org = req.orgID()
	case *op.DeviceAuthorizationState:
		d := requestInfo(ctx).device
		if d == nil {
			return nil, errNoFlow
		}
		org = d.orgID
	default:
		return nil, errNoFlow
	}
	return map[string]any{claimOrg: org.String()}, nil
}
