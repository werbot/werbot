package identity

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/identity/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/platform/sshca"
)

// UserCA returns the organization's ed25519 user CA, creating it on first
// use. The seed is stored sealed by Box; concurrent first uses on any replica
// converge on one key: the first insert wins, the rest re-read.
func (s *Service) UserCA(ctx context.Context, orgID uuid.UUID) (ssh.Signer, error) {
	if ca, ok := s.cas.Load(orgID); ok {
		return ca.(ssh.Signer), nil
	}
	var row dbq.GetUserCARow
	err := dbx.InOrg(ctx, s.pool, orgID, uuid.Nil, func(tx pgx.Tx) error {
		st := newStore(tx)
		var err error
		if row, err = st.GetUserCA(ctx, orgID); !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err = s.createUserCA(ctx, st, orgID); err != nil {
			return err
		}
		row, err = st.GetUserCA(ctx, orgID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("user CA: %w", err)
	}
	seed, err := s.cfg.Box.Open(row.Ciphertext, row.KeyID)
	if err != nil {
		return nil, fmt.Errorf("user CA: %w", err)
	}
	if len(seed) != ed25519.SeedSize {
		return nil, errors.New("user CA: stored key is not an ed25519 seed")
	}
	ca, err := ssh.NewSignerFromKey(ed25519.NewKeyFromSeed(seed))
	if err != nil {
		return nil, fmt.Errorf("user CA: %w", err)
	}
	got, _ := s.cas.LoadOrStore(orgID, ca)
	return got.(ssh.Signer), nil
}

func (s *Service) createUserCA(ctx context.Context, st Store, orgID uuid.UUID) error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return err
	}
	ct, keyID, err := s.cfg.Box.Seal(priv.Seed())
	if err != nil {
		return err
	}
	return st.InsertUserCA(ctx, dbq.InsertUserCAParams{OrgID: orgID, Ciphertext: ct, KeyID: keyID, PublicKey: authorizedKey(sshPub)})
}

// authorizedKey is k in authorized_keys format without the trailing newline.
func authorizedKey(k ssh.PublicKey) string {
	return strings.TrimSuffix(string(ssh.MarshalAuthorizedKey(k)), "\n")
}

// UserCAPublicKeys returns the organization's user CA keys for the gateway
// snapshot, creating the CA on first use: the first snapshot already carries it.
func (s *Service) UserCAPublicKeys(ctx context.Context, orgID uuid.UUID) ([]string, error) {
	ca, err := s.UserCA(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return []string{authorizedKey(ca.PublicKey())}, nil
}

// IssueUserCert signs pub, an ed25519 key, for the CLI session of p for
// sshca.UserCertTTL. Only a CLI token (AuthMethodOAuth) gets a certificate.
// Issuing is counted, not audited: the CLI renews certificates often.
func (s *Service) IssueUserCert(ctx context.Context, p principal.Principal, pub ssh.PublicKey) (*ssh.Certificate, error) {
	if p.AuthMethod != principal.AuthMethodOAuth {
		return nil, fmt.Errorf("user certificates need a CLI token: %w", apperr.ErrPermissionDenied)
	}
	if pub.Type() != ssh.KeyAlgoED25519 {
		return nil, fmt.Errorf("public key must be ed25519: %w", apperr.ErrInvalidArgument)
	}
	var sub dbq.GetCertSubjectRow
	err := dbx.ReadInOrg(s.pool, p.OrgID, p.UserID, func(rd *dbx.Read) (err error) {
		sub, err = newStore(rd).GetCertSubject(ctx, dbq.GetCertSubjectParams{OrgID: p.OrgID, CliSessionID: p.CLISessionID, UserID: p.UserID})
		return err
	})
	if err != nil {
		return nil, err
	}
	if sub.Locked {
		return nil, fmt.Errorf("cli session is locked: %w", apperr.ErrPermissionDenied)
	}
	ca, err := s.UserCA(ctx, p.OrgID)
	if err != nil {
		return nil, err
	}
	cert, err := sshca.Sign(ca, pub, sshca.Identity{OrgID: p.OrgID, UserID: p.UserID, CLISessionID: p.CLISessionID, Login: sub.Login},
		time.Now(), sshca.UserCertTTL)
	if err != nil {
		return nil, err
	}
	s.certsIssued.Inc()
	return cert, nil
}

// AuthenticateCert resolves a user certificate the gateway of orgID verified:
// its cli_session must be live in orgID, its user a member whose login is the
// certificate's principal. Anything else is ErrNotFound.
func (s *Service) AuthenticateCert(ctx context.Context, orgID uuid.UUID, id sshca.Identity) (UserRef, error) {
	return s.lookupRef(orgID, func(st Store) (dbq.LookupLoginUserRow, error) {
		r, err := st.LookupCertUser(ctx, dbq.LookupCertUserParams{ID: id.CLISessionID, OrgID: orgID, UserID: id.UserID, Login: id.Login})
		return dbq.LookupLoginUserRow(r), err
	})
}
