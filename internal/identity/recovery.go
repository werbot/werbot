package identity

import (
	"context"
	"crypto/rand"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/identity/dbq"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// RecoveryCodeCount is the size of a set of recovery codes.
const RecoveryCodeCount = 10

// GenerateRecoveryCodes replaces p's recovery codes with a new set and
// returns it; this is the only time the codes are shown. Each code is 80
// random bits in lowercase base32, xxxx-xxxx-xxxx-xxxx; only its SHA-256 is stored.
func (s *Service) GenerateRecoveryCodes(ctx context.Context, p principal.Principal) ([]string, error) {
	codes := make([]string, RecoveryCodeCount)
	hashes := make([][]byte, RecoveryCodeCount)
	for i := range codes {
		raw := strings.ToLower(rand.Text()[:16]) // 16 base32 characters = 80 bits
		codes[i] = raw[:4] + "-" + raw[4:8] + "-" + raw[8:12] + "-" + raw[12:]
		hashes[i] = hashSecret(raw)
	}
	err := dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		st := newStore(tx)
		if _, err := st.DeleteRecoveryCodes(ctx, p.UserID); err != nil {
			return err
		}
		if err := st.InsertRecoveryCodes(ctx, dbq.InsertRecoveryCodesParams{UserID: p.UserID, CodeHashes: hashes}); err != nil {
			return err
		}
		return writeFactorAudit(ctx, tx, "mfa.factor.add", p, factorRecovery, factorRecovery)
	})
	if err != nil {
		return nil, err
	}
	return codes, nil
}

// useRecoveryCode spends one unused recovery code of userID in tx; case,
// dashes and spaces are ignored. Like checkTOTP it does not count the
// attempt: the caller counts mfa:<user_id> before it opens tx.
func (s *Service) useRecoveryCode(ctx context.Context, tx pgx.Tx, userID uuid.UUID, code string) error {
	n, err := newStore(tx).UseRecoveryCode(ctx, dbq.UseRecoveryCodeParams{UserID: userID, CodeHash: hashSecret(normalizeRecoveryCode(code))})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrInvalidCode
	}
	return nil
}

func normalizeRecoveryCode(code string) string {
	return strings.Map(func(r rune) rune {
		if r == '-' || unicode.IsSpace(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, code)
}
