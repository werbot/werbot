package oauth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/werbot/werbot/internal/identity/oauth/dbq"
	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/dbx"
)

// Refresh tokens are stored only as SHA-256. Each refresh rotates the token
// and keeps the previous hash: that token coming back is a replay and ends
// the cli_session.

var (
	errRefreshUnknown = errors.New("refresh token unknown or ended") // op answers invalid_grant
	errRefreshReuse   = errors.New("refresh token reused: cli_session revoked")
)

// refreshRequest is a live cli_session found by its refresh token.
type refreshRequest struct {
	dbq.GetLiveCLISessionRow
	scopes []string
}

var _ op.RefreshTokenRequest = (*refreshRequest)(nil)

func (*refreshRequest) GetAMR() []string               { return nil }
func (*refreshRequest) GetAudience() []string          { return []string{Audience} }
func (*refreshRequest) GetAuthTime() time.Time         { return time.Time{} }
func (r *refreshRequest) GetClientID() string          { return r.ClientID }
func (r *refreshRequest) GetScopes() []string          { return r.scopes }
func (r *refreshRequest) GetSubject() string           { return r.UserID.String() }
func (r *refreshRequest) SetCurrentScopes(sc []string) { r.scopes = sc }

// TokenRequestByRefreshToken implements op.Storage: the session of a live
// token (not revoked, before expires_at, absolute_expires_at and created_at
// + cli_session_max of the current settings). The previous token of a session
// revokes it (refresh_reuse, audited as cli_session.revoke by the system).
// Every error is invalid_grant.
func (s *storage) TokenRequestByRefreshToken(ctx context.Context, token string) (op.RefreshTokenRequest, error) {
	hash := hashSecret(token)
	var (
		req    *refreshRequest
		reused bool
	)
	err := dbx.System(ctx, s.pool, dbx.ReasonLogin, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		row, err := q.GetLiveCLISession(ctx, hash)
		if err == nil {
			req = &refreshRequest{GetLiveCLISessionRow: row, scopes: []string{oidc.ScopeOpenID, oidc.ScopeOfflineAccess}}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		r, err := q.RevokeReusedCLISession(ctx, hash)
		if errors.Is(err, pgx.ErrNoRows) {
			return errRefreshUnknown
		}
		if err != nil {
			return err
		}
		reused = true // commit the revoke, then refuse
		return auditlog.Write(ctx, tx, auditlog.Event{
			OrgID: &r.OrgID, ActorKind: "system", Action: "cli_session.revoke",
			TargetType: "cli_session", TargetID: r.ID.String(), IP: requestInfo(ctx).ip,
			Data: map[string]any{"reason": "refresh_reuse"},
		})
	})
	switch {
	case err != nil:
		return nil, err
	case reused:
		return nil, errRefreshReuse
	}
	return req, nil
}

// rotate replaces the refresh token of req's session in one statement and
// slides expires_at to least(now + cli_session_ttl, absolute_expires_at,
// created_at + cli_session_max). Of concurrent rotations of one token only
// one matches; the others get invalid_grant and the session stays live.
func (s *storage) rotate(ctx context.Context, req *refreshRequest, current string) (string, string, time.Time, error) {
	raw, hash := newRefreshToken()
	var n int64
	err := dbx.System(ctx, s.pool, dbx.ReasonLogin, func(tx pgx.Tx) error {
		var err error
		n, err = dbq.New(tx).RotateCLISession(ctx, dbq.RotateCLISessionParams{NewHash: hash, ID: req.ID, RefreshHash: hashSecret(current)})
		return err
	})
	if err != nil {
		return "", "", time.Time{}, err
	}
	if n == 0 {
		return "", "", time.Time{}, oidc.ErrInvalidGrant().WithDescription("refresh token already used")
	}
	return req.ID.String(), raw, time.Now().Add(AccessTokenTTL), nil
}

// GetRefreshTokenInfo implements op.AuthStorage for /oauth/revoke: the user
// and cli_session of a refresh token. An unknown token is
// op.ErrInvalidRefreshToken, so op tries it as an access token (jti).
func (s *storage) GetRefreshTokenInfo(ctx context.Context, clientID, token string) (userID, tokenID string, err error) {
	var row dbq.GetCLISessionByRefreshRow
	err = dbx.System(ctx, s.pool, dbx.ReasonLogin, func(tx pgx.Tx) error {
		var err error
		row, err = dbq.New(tx).GetCLISessionByRefresh(ctx, dbq.GetCLISessionByRefreshParams{RefreshHash: hashSecret(token), ClientID: clientID})
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", op.ErrInvalidRefreshToken
	}
	if err != nil {
		return "", "", err
	}
	return row.UserID.String(), row.ID.String(), nil
}

// RevokeToken implements op.AuthStorage: tokenOrID is a cli_session id and
// userID its user when op matched the token (refresh token or access token
// jti); the session ends as logout, audited as cli.logout. Anything else,
// the raw token included, is not an error (RFC 7009 §2.2).
func (s *storage) RevokeToken(ctx context.Context, tokenOrID, userID, _ string) *oidc.Error {
	id, err1 := parseID(tokenOrID)
	user, err2 := parseID(userID)
	if err1 != nil || err2 != nil {
		return nil
	}
	err := dbx.System(ctx, s.pool, dbx.ReasonLogin, func(tx pgx.Tx) error {
		org, err := dbq.New(tx).LogoutCLISession(ctx, dbq.LogoutCLISessionParams{ID: id, UserID: user})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // already ended
		}
		if err != nil {
			return err
		}
		return auditlog.Write(ctx, tx, auditlog.Event{
			OrgID: &org, ActorUserID: &user, ActorKind: "user", Action: "cli.logout",
			TargetType: "cli_session", TargetID: id.String(), IP: requestInfo(ctx).ip,
		})
	})
	if err != nil {
		return oidc.ErrServerError().WithParent(err)
	}
	return nil
}
