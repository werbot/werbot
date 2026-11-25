package identity

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"image/color"
	"strings"
	"time"

	"github.com/boombuler/barcode/qr"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"github.com/werbot/werbot/internal/identity/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// TOTP codes (spec D §10): SHA-1, 6 digits, 30-second steps, one step of
// clock skew either way.
const (
	TOTPPeriod = 30 * time.Second
	TOTPSkew   = 1
)

// ErrInvalidCode answers a wrong, reused or expired code of any factor.
var ErrInvalidCode = fmt.Errorf("invalid code: %w", apperr.ErrPermissionDenied)

var totpOpts = totp.ValidateOpts{Period: uint(TOTPPeriod / time.Second), Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1}

// TOTPEnrollment is shown once: the otpauth URL, its base32 secret for manual
// entry and its QR code, QRSize × QRSize modules drawn by the SVG path QRPath.
type TOTPEnrollment struct {
	URL, Secret string
	QRSize      int
	QRPath      string
}

// EnrollTOTP stores a new sealed TOTP secret of p's user, replacing an
// unconfirmed one; a confirmed secret is ErrAlreadyExists.
func (s *Service) EnrollTOTP(ctx context.Context, p principal.Principal) (TOTPEnrollment, error) {
	var e TOTPEnrollment
	err := dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		st := newStore(tx)
		u, err := st.GetProfile(ctx, p.UserID)
		if err != nil {
			return err
		}
		// The defaults are the parameters of totpOpts and a 20-byte secret.
		key, err := totp.Generate(totp.GenerateOpts{Issuer: "Werbot", AccountName: u.Email})
		if err != nil {
			return err
		}
		ct, keyID, err := s.cfg.Box.Seal([]byte(key.Secret()))
		if err != nil {
			return err
		}
		n, err := st.UpsertTOTPSecret(ctx, dbq.UpsertTOTPSecretParams{UserID: p.UserID, Ciphertext: ct, KeyID: keyID})
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("totp is already enrolled: %w", apperr.ErrAlreadyExists)
		}
		e = TOTPEnrollment{URL: key.URL(), Secret: key.Secret()}
		return nil
	})
	if err != nil {
		return TOTPEnrollment{}, err
	}
	e.QRSize, e.QRPath, err = qrPath(e.URL)
	return e, err
}

// ConfirmTOTP turns the pending enrollment into a factor with a current code
// and makes a browser session fresh. The attempt is counted against mfa:<user_id> first; a wrong code leaves the
// enrollment pending and is audited as auth.mfa.fail.
func (s *Service) ConfirmTOTP(ctx context.Context, p principal.Principal, code string) error {
	if err := s.limit(ctx, "mfa:"+p.UserID.String(), MFAAttempts, MFAWindow); err != nil {
		return err
	}
	err := dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		n, err := newStore(tx).ConfirmTOTPSecret(ctx, p.UserID)
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("no totp enrollment to confirm: %w", apperr.ErrFailedPrecondition)
		}
		if err := s.checkTOTP(ctx, tx, p.UserID, code); err != nil {
			return err
		}
		if err := enrolled(ctx, tx, p, factorTOTP); err != nil {
			return err
		}
		return writeFactorAudit(ctx, tx, "mfa.factor.add", p, factorTOTP, factorTOTP)
	})
	if errors.Is(err, ErrInvalidCode) {
		return errors.Join(err, s.auditMFAFail(ctx, p.OrgID, p.UserID, factorTOTP))
	}
	return err
}

// checkTOTP spends code of userID's confirmed TOTP secret in tx: the code of
// step t−1, t or t+1 is accepted once and only if that step is newer than
// last_step. It does not count the attempt: every caller counts mfa:<user_id>
// with limit before it opens tx.
func (s *Service) checkTOTP(ctx context.Context, tx pgx.Tx, userID uuid.UUID, code string) error {
	st := newStore(tx)
	row, err := st.GetTOTPSecret(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidCode
	}
	if err != nil {
		return err
	}
	secret, err := s.cfg.Box.Open(row.Ciphertext, row.KeyID)
	if err != nil {
		return err
	}
	step, ok := matchStep(string(secret), code, time.Now())
	if !ok {
		return ErrInvalidCode
	}
	n, err := st.UseTOTPStep(ctx, dbq.UseTOTPStepParams{UserID: userID, Step: step})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrInvalidCode
	}
	return nil
}

// matchStep returns the step within TOTPSkew of now whose code is code.
func matchStep(secret, code string, now time.Time) (int64, bool) {
	period := int64(TOTPPeriod / time.Second)
	t := now.Unix() / period
	for step := t - TOTPSkew; step <= t+TOTPSkew; step++ {
		want, err := totp.GenerateCodeCustom(secret, time.Unix(step*period, 0), totpOpts)
		if err == nil && subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			return step, true
		}
	}
	return 0, false
}

// qrPath encodes content as a QR code of level M and draws it as one SVG path
// of 1×1 squares (runs of dark modules merged per row) on a size × size grid:
// the page CSP allows no data: or blob: images.
func qrPath(content string) (size int, path string, err error) {
	code, err := qr.Encode(content, qr.M, qr.Auto)
	if err != nil {
		return 0, "", err
	}
	size = code.Bounds().Dx()
	var b strings.Builder
	for y := range size {
		for x := 0; x < size; x++ {
			run := 0
			for x+run < size && code.At(x+run, y) == color.Black {
				run++
			}
			if run > 0 {
				fmt.Fprintf(&b, "M%d %dh%dv1h-%dz", x, y, run, run)
				x += run
			}
		}
	}
	return size, b.String(), nil
}
