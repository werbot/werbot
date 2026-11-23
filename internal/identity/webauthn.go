package identity

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/identity/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// CeremonyTTL is how long a WebAuthn ceremony can be finished.
const CeremonyTTL = 5 * time.Minute

const maxPasskeyName = 64 // webauthn_credential.name CHECK

var (
	errPasskeysOff  = fmt.Errorf("passkeys need a domain name in WERBOT_WEB_URL: %w", apperr.ErrFailedPrecondition)
	errDeviceLocked = fmt.Errorf("mfa device is locked: %w", apperr.ErrPermissionDenied)
)

// newWebAuthn returns the relying party of cfg.WebURL: the RP ID is its host
// without the port, the origin the URL itself. Browsers and go-webauthn refuse
// an IP address as RP ID: then passkeys are off (nil) and TOTP alone works.
func newWebAuthn(cfg Config) *webauthn.WebAuthn {
	var host string
	if u, err := url.Parse(cfg.WebURL); err == nil {
		host = u.Hostname()
	}
	wa, err := webauthn.New(&webauthn.Config{RPID: host, RPDisplayName: "Werbot", RPOrigins: []string{cfg.WebURL}})
	if err != nil {
		cmp.Or(cfg.Logger, slog.Default()).Warn("passkeys are off: they need a domain name in WERBOT_WEB_URL", "error", err)
		return nil
	}
	return wa
}

// passkeyUser is a user as go-webauthn sees it: the 16 bytes of the user id,
// the email as name and the passkeys; ids maps a credential id to its
// webauthn_credential row.
type passkeyUser struct {
	id    uuid.UUID
	name  string
	creds []webauthn.Credential
	ids   map[string]uuid.UUID
}

func (u passkeyUser) WebAuthnID() []byte                         { return u.id[:] }
func (u passkeyUser) WebAuthnName() string                       { return u.name }
func (u passkeyUser) WebAuthnDisplayName() string                { return u.name }
func (u passkeyUser) WebAuthnCredentials() []webauthn.Credential { return u.creds }

func loadPasskeys(ctx context.Context, st Store, userID uuid.UUID) (passkeyUser, error) {
	rows, err := st.ListWebAuthnCredentials(ctx, userID)
	u := passkeyUser{id: userID, ids: make(map[string]uuid.UUID, len(rows))}
	for _, r := range rows {
		c := webauthn.Credential{ID: r.CredentialID, PublicKey: r.PublicKey, Flags: webauthn.CredentialFlags{BackupEligible: r.BackupEligible},
			Authenticator: webauthn.Authenticator{AAGUID: r.Aaguid, SignCount: uint32(r.SignCount)}} //nolint:gosec // stored from a uint32
		for _, t := range r.Transports {
			c.Transport = append(c.Transport, protocol.AuthenticatorTransport(t))
		}
		u.creds = append(u.creds, c)
		u.ids[string(r.CredentialID)] = r.ID
	}
	return u, err
}

// BeginWebAuthnRegistration starts a passkey registration of p's user and
// returns the creation options; the user's passkeys are excluded.
func (s *Service) BeginWebAuthnRegistration(ctx context.Context, p principal.Principal) ([]byte, error) {
	if s.wa == nil {
		return nil, errPasskeysOff
	}
	var opts []byte
	err := dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		st := newStore(tx)
		u, err := loadPasskeys(ctx, st, p.UserID)
		if err != nil {
			return err
		}
		prof, err := st.GetProfile(ctx, p.UserID)
		if err != nil {
			return err
		}
		u.name = prof.Email
		creation, session, err := s.wa.BeginRegistration(u, webauthn.WithExclusions(webauthn.Credentials(u.creds).CredentialDescriptors()))
		if err != nil {
			return err
		}
		opts, err = saveCeremony(ctx, st, ceremony{orgID: p.OrgID, userID: p.UserID, kind: "register"}, CeremonyTTL, creation, session)
		return err
	})
	return opts, err
}

// FinishWebAuthnRegistration stores the passkey of attestation (the
// browser's JSON) as name and audits mfa.factor.add. An attestation that
// does not answer p's open registration ceremony is ErrInvalidCode.
func (s *Service) FinishWebAuthnRegistration(ctx context.Context, p principal.Principal, name string, attestation []byte) (Factor, error) {
	if s.wa == nil {
		return Factor{}, errPasskeysOff
	}
	name, err := passkeyName(name)
	if err != nil {
		return Factor{}, err
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(attestation)
	if err != nil {
		return Factor{}, ErrInvalidCode
	}
	var f Factor
	err = dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) (err error) {
		f, err = s.storePasskey(ctx, tx, p, name, parsed)
		return err
	})
	if err != nil {
		return Factor{}, err
	}
	return f, nil
}

func (s *Service) storePasskey(ctx context.Context, tx pgx.Tx, p principal.Principal, name string, parsed *protocol.ParsedCredentialCreationData) (Factor, error) {
	st := newStore(tx)
	session, err := useCeremony(ctx, st, ceremony{orgID: p.OrgID, userID: p.UserID, kind: "register"})
	if err != nil {
		return Factor{}, err
	}
	cred, err := s.wa.CreateCredential(passkeyUser{id: p.UserID}, session, parsed)
	if err != nil {
		return Factor{}, ErrInvalidCode
	}
	transports := make([]string, 0, len(cred.Transport)) // the column is NOT NULL
	for _, t := range cred.Transport {
		transports = append(transports, string(t))
	}
	row, err := st.CreateWebAuthnCredential(ctx, dbq.CreateWebAuthnCredentialParams{UserID: p.UserID, Name: name, CredentialID: cred.ID,
		PublicKey: cred.PublicKey, SignCount: int64(cred.Authenticator.SignCount), Aaguid: cred.Authenticator.AAGUID, Transports: transports,
		BackupEligible: cred.Flags.BackupEligible})
	if isUniqueViolation(err) {
		return Factor{}, fmt.Errorf("passkey is already registered: %w", apperr.ErrAlreadyExists)
	}
	if err != nil {
		return Factor{}, err
	}
	f := Factor{ID: row.ID.String(), Kind: factorWebAuthn, Name: name, CreatedAt: row.CreatedAt}
	return f, writeFactorAudit(ctx, tx, "mfa.factor.add", p, factorWebAuthn, f.ID)
}

// BeginWebAuthnLogin starts an assertion ceremony of userID in orgID that
// lives ttl and returns the request options; kind (login, stepup or
// challenge) and ref bind it to what it confirms. A user without a passkey
// is ErrFailedPrecondition.
func (s *Service) BeginWebAuthnLogin(ctx context.Context, orgID, userID uuid.UUID, kind, ref string, ttl time.Duration) ([]byte, error) {
	if s.wa == nil {
		return nil, errPasskeysOff
	}
	var opts []byte
	err := dbx.InOrg(ctx, s.pool, orgID, userID, func(tx pgx.Tx) error {
		st := newStore(tx)
		u, err := loadPasskeys(ctx, st, userID)
		if err != nil {
			return err
		}
		if len(u.creds) == 0 {
			return fmt.Errorf("no passkey: %w", apperr.ErrFailedPrecondition)
		}
		assertion, session, err := s.wa.BeginLogin(u)
		if err != nil {
			return err
		}
		opts, err = saveCeremony(ctx, st, ceremony{orgID: orgID, userID: userID, kind: kind, ref: ref}, ttl, assertion, session)
		return err
	})
	return opts, err
}

// FinishWebAuthnLogin verifies assertion (the browser's JSON) against the
// open ceremony of userID, kind and ref in the caller's effect transaction tx
// and returns the passkey's id. A wrong or replayed assertion, another
// origin or RP ID and a sign count that did not grow (a cloned
// authenticator) are ErrInvalidCode; a passkey under an active mfa_device
// lock of orgID is refused. The attempt is not counted against mfa:<user_id>.
func (s *Service) FinishWebAuthnLogin(ctx context.Context, tx pgx.Tx, orgID, userID uuid.UUID, kind, ref string, assertion []byte) (uuid.UUID, error) {
	if s.wa == nil {
		return uuid.Nil, errPasskeysOff
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(assertion)
	if err != nil {
		return uuid.Nil, ErrInvalidCode
	}
	st := newStore(tx)
	session, err := useCeremony(ctx, st, ceremony{orgID: orgID, userID: userID, kind: kind, ref: ref})
	if err != nil {
		return uuid.Nil, err
	}
	id, count, err := s.verifyAssertion(ctx, st, userID, session, parsed)
	if err != nil {
		return uuid.Nil, err
	}
	if err := useCredential(ctx, st, orgID, id, count); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (s *Service) verifyAssertion(ctx context.Context, st Store, userID uuid.UUID, session webauthn.SessionData, parsed *protocol.ParsedCredentialAssertionData) (uuid.UUID, int64, error) {
	u, err := loadPasskeys(ctx, st, userID)
	if err != nil {
		return uuid.Nil, 0, err
	}
	cred, err := s.wa.ValidateLogin(u, session, parsed)
	if err != nil || cred.Authenticator.CloneWarning { // go-webauthn flags a clone without an error
		return uuid.Nil, 0, ErrInvalidCode
	}
	return u.ids[string(cred.ID)], int64(cred.Authenticator.SignCount), nil
}

// useCredential refuses a locked passkey and stores its new sign count; a
// concurrent assertion that stored a higher count first is ErrInvalidCode.
func useCredential(ctx context.Context, st Store, orgID, id uuid.UUID, count int64) error {
	locked, err := st.MFADeviceLocked(ctx, dbq.MFADeviceLockedParams{OrgID: orgID, ID: id})
	if err != nil {
		return err
	}
	if locked {
		return errDeviceLocked
	}
	n, err := st.UseWebAuthnCredential(ctx, dbq.UseWebAuthnCredentialParams{ID: id, SignCount: count})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrInvalidCode
	}
	return nil
}

// passkeyName trims name and checks the 1..maxPasskeyName rune range.
func passkeyName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxPasskeyName {
		return "", fmt.Errorf("name must have 1 to %d characters: %w", maxPasskeyName, apperr.ErrInvalidArgument)
	}
	return name, nil
}
