package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/identity/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
)

// Kinds of one-time token.
const (
	tokenInvite          = "invite"
	tokenPasswordReset   = "password_reset"
	tokenAccountDeletion = "account_deletion"
)

var tokenTTL = map[string]time.Duration{
	tokenInvite:          7 * 24 * time.Hour,
	tokenPasswordReset:   time.Hour,
	tokenAccountDeletion: time.Hour,
}

// tokenClaims is what a token grants; empty fields are stored as NULL.
type tokenClaims struct {
	OrgID, UserID *uuid.UUID
	Email, Role   string
}

// issueToken stores a new token of kind and returns its raw value, which
// exists nowhere else: the database keeps only its hash.
func issueToken(ctx context.Context, st Store, kind string, c tokenClaims) (string, error) {
	ttl, ok := tokenTTL[kind]
	if !ok {
		return "", fmt.Errorf("unknown token kind %q", kind)
	}
	raw, hash := newSecret()
	err := st.CreateOneTimeToken(ctx, dbq.CreateOneTimeTokenParams{
		TokenHash: hash, Kind: kind, OrgID: c.OrgID, UserID: c.UserID,
		Email: nullable(c.Email), Role: nullable(c.Role), TtlSeconds: ttl.Seconds(),
	})
	if err != nil {
		return "", err
	}
	return raw, nil
}

// consumeToken marks the token used and returns its claims. Run it in the
// transaction of the effect: if the effect fails, the token is not spent.
// An unknown, wrong-kind, used or expired token is ErrNotFound.
func consumeToken(ctx context.Context, st Store, kind, raw string) (tokenClaims, error) {
	row, err := st.ConsumeOneTimeToken(ctx, dbq.ConsumeOneTimeTokenParams{TokenHash: hashSecret(raw), Kind: kind})
	if errors.Is(err, pgx.ErrNoRows) {
		return tokenClaims{}, fmt.Errorf("invalid or expired token: %w", apperr.ErrNotFound)
	}
	if err != nil {
		return tokenClaims{}, err
	}
	c := tokenClaims{OrgID: row.OrgID, UserID: row.UserID}
	if row.Email != nil {
		c.Email = *row.Email
	}
	if row.Role != nil {
		c.Role = *row.Role
	}
	return c, nil
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
