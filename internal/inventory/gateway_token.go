package inventory

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/inventory/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

const joinTokenPrefix = "wjt_"

// JoinToken is a reusable pool enrollment token; the secret is never stored.
type JoinToken struct {
	ID                   uuid.UUID
	MaxUses, Uses        int
	ExpiresAt, CreatedAt time.Time
	RevokedAt            *time.Time
	CreatedBy            *uuid.UUID
}

// newJoinToken returns wjt_<secret>.<CA fingerprint> and the SHA-256 of the secret.
func (g *Gateways) newJoinToken() (token string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	secret := base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(secret))
	return joinTokenPrefix + secret + "." + g.ca.Fingerprint(), sum[:], nil
}

// parseJoinToken returns the hash of the token secret and the CA fingerprint; ok is false when malformed.
func parseJoinToken(token string) (hash []byte, fingerprint string, ok bool) {
	rest, ok := strings.CutPrefix(token, joinTokenPrefix)
	i := strings.LastIndexByte(rest, '.')
	if !ok || i < 0 {
		return nil, "", false
	}
	sum := sha256.Sum256([]byte(rest[:i]))
	return sum[:], rest[i+1:], true
}

// issueJoinToken stores a new token of gatewayID (ErrNotFound outside orgID) and audits it.
func (g *Gateways) issueJoinToken(ctx context.Context, st Store, tx pgx.Tx, a actor, orgID, gatewayID uuid.UUID,
	ttl time.Duration, maxUses int32,
) (id uuid.UUID, token string, expiresAt time.Time, err error) {
	token, hash, err := g.newJoinToken()
	if err != nil {
		return uuid.Nil, "", time.Time{}, err
	}
	row, err := st.CreateJoinToken(ctx, dbq.CreateJoinTokenParams{TokenHash: hash, OrgID: orgID, GatewayID: gatewayID,
		MaxUses: maxUses, TtlSeconds: ttl.Seconds(), CreatedBy: a.user})
	if isForeignKeyViolation(err) {
		return uuid.Nil, "", time.Time{}, errGatewayNotFound
	}
	if err != nil {
		return uuid.Nil, "", time.Time{}, err
	}
	err = a.audit(ctx, tx, orgID, "gateway.join_token.create", "gateway_join_token", row.ID, map[string]any{
		"gateway_id": gatewayID.String(), "token_id": row.ID.String(), "max_uses": maxUses, "expires_at": row.ExpiresAt})
	if err != nil {
		return uuid.Nil, "", time.Time{}, err
	}
	return row.ID, token, row.ExpiresAt, nil
}

// CreateJoinToken issues a reusable token for the pool; zero ttl or maxUses takes the default.
// The token is returned only here.
func (g *Gateways) CreateJoinToken(ctx context.Context, gatewayID uuid.UUID, ttl time.Duration, maxUses int) (uuid.UUID, string, time.Time, error) {
	if ttl == 0 {
		ttl = JoinTokenTTLDefault
	}
	if maxUses == 0 {
		maxUses = JoinTokenMaxUsesDefault
	}
	if ttl < JoinTokenTTLMin || ttl > JoinTokenTTLMax {
		return uuid.Nil, "", time.Time{}, invalid("join token TTL must be between %v and %v", JoinTokenTTLMin, JoinTokenTTLMax)
	}
	if maxUses < 1 || maxUses > JoinTokenMaxUsesMax {
		return uuid.Nil, "", time.Time{}, invalid("join token max uses must be between 1 and %d", JoinTokenMaxUsesMax)
	}
	uses := int32(maxUses)
	var (
		id        uuid.UUID
		token     string
		expiresAt time.Time
	)
	err := g.admin(ctx, func(p principal.Principal, st Store, tx pgx.Tx) (err error) {
		id, token, expiresAt, err = g.issueJoinToken(ctx, st, tx, userActor(ctx, p), p.OrgID, gatewayID, ttl, uses)
		return err
	})
	if err != nil {
		return uuid.Nil, "", time.Time{}, err
	}
	return id, token, expiresAt, nil
}

// ListJoinTokens returns the tokens of a pool, oldest first.
func (g *Gateways) ListJoinTokens(ctx context.Context, gatewayID uuid.UUID) ([]JoinToken, error) {
	var out []JoinToken
	err := g.admin(ctx, func(p principal.Principal, st Store, _ pgx.Tx) error {
		rows, err := st.ListJoinTokens(ctx, dbq.ListJoinTokensParams{GatewayID: gatewayID, OrgID: p.OrgID})
		for _, r := range rows {
			out = append(out, JoinToken{ID: r.ID, MaxUses: int(r.MaxUses), Uses: int(r.Uses), ExpiresAt: r.ExpiresAt,
				CreatedAt: r.CreatedAt, RevokedAt: r.RevokedAt, CreatedBy: r.CreatedBy})
		}
		return err
	})
	return out, err
}

// RevokeJoinToken stops a token from enrolling; revoking twice keeps the
// first time and audits only the first.
func (g *Gateways) RevokeJoinToken(ctx context.Context, id uuid.UUID) error {
	return g.admin(ctx, func(p principal.Principal, st Store, tx pgx.Tx) error {
		row, err := st.RevokeJoinToken(ctx, dbq.RevokeJoinTokenParams{ID: id, OrgID: p.OrgID})
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("join token not found: %w", apperr.ErrNotFound)
		}
		if err != nil || !row.RevokedNow {
			return err
		}
		return userActor(ctx, p).audit(ctx, tx, p.OrgID, "gateway.join_token.revoke", "gateway_join_token", id,
			map[string]any{"gateway_id": row.GatewayID.String()})
	})
}

// CreateSystemJoinToken issues a single-use token of the pool "local" of
// orgID for the built-in gateway, creating the pool on first use.
func (g *Gateways) CreateSystemJoinToken(ctx context.Context, orgID uuid.UUID) (string, error) {
	sys := actor{kind: "system"}
	var token string
	err := dbx.InOrg(ctx, g.pool, orgID, uuid.Nil, func(tx pgx.Tx) error {
		st := newStore(tx)
		id, err := st.GetGatewayIDByName(ctx, dbq.GetGatewayIDByNameParams{OrgID: orgID, Name: SystemPoolName})
		if errors.Is(err, pgx.ErrNoRows) {
			id, err = g.createSystemPool(ctx, st, tx, sys, orgID)
		}
		if err != nil {
			return err
		}
		_, token, _, err = g.issueJoinToken(ctx, st, tx, sys, orgID, id, SystemJoinTokenTTL, 1)
		return err
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// createSystemPool creates the pool "local"; a concurrent creator may win the race.
func (g *Gateways) createSystemPool(ctx context.Context, st Store, tx pgx.Tx, sys actor, orgID uuid.UUID) (uuid.UUID, error) {
	ct, keyID, err := g.newHostKey()
	if err != nil {
		return uuid.Nil, err
	}
	id, err := st.CreateGatewayIfAbsent(ctx, dbq.CreateGatewayIfAbsentParams{
		OrgID: orgID, Name: SystemPoolName, HostKeyCiphertext: ct, HostKeyKeyID: keyID})
	if errors.Is(err, pgx.ErrNoRows) {
		return st.GetGatewayIDByName(ctx, dbq.GetGatewayIDByNameParams{OrgID: orgID, Name: SystemPoolName})
	}
	if err != nil {
		return uuid.Nil, err
	}
	return id, poolCreated(ctx, st, tx, sys, orgID, id, SystemPoolName)
}
