package identity

import (
	"cmp"
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/identity/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

const (
	minRSABits = 2048
	algoDSA    = "ssh-dss" // ssh.KeyAlgoDSA is deprecated
	maxKeyName = 128       // user_ssh_key.name CHECK
)

// SSHKey is a public key a user signs in to the gateway with.
type SSHKey struct {
	ID                uuid.UUID
	Name              string
	PublicKey         string
	FingerprintSHA256 string
	CreatedAt         time.Time
}

func keyFromRow(r dbq.ListSSHKeysRow) SSHKey {
	return SSHKey{ID: r.ID, Name: r.Name, PublicKey: r.PublicKey, FingerprintSHA256: r.FingerprintSha256, CreatedAt: r.CreatedAt}
}

// keyName trims name and checks the 1..maxKeyName rune range of the DB CHECK.
func keyName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", invalidKey("name is required")
	}
	if utf8.RuneCountInString(name) > maxKeyName {
		return "", invalidKey("name is too long")
	}
	return name, nil
}

func invalidKey(msg string) error { return fmt.Errorf("%s: %w", msg, apperr.ErrInvalidArgument) }

// parseSSHKey parses exactly one authorized_keys public key. Errors never
// include the input, so a pasted private key cannot leak into logs.
func parseSSHKey(text string) (ssh.PublicKey, string, error) {
	if strings.Contains(text, "PRIVATE KEY") {
		return nil, "", invalidKey("private keys are not accepted")
	}
	key, comment, _, rest, err := ssh.ParseAuthorizedKey([]byte(text))
	if err != nil || strings.TrimSpace(string(rest)) != "" {
		return nil, "", invalidKey("expected exactly one public key in authorized_keys format")
	}
	if _, ok := key.(*ssh.Certificate); ok {
		return nil, "", invalidKey("certificates are not accepted")
	}
	if key.Type() == algoDSA {
		return nil, "", invalidKey("ssh-dss keys are not accepted")
	}
	if ck, ok := key.(ssh.CryptoPublicKey); ok {
		if pk, ok := ck.CryptoPublicKey().(*rsa.PublicKey); ok && pk.N.BitLen() < minRSABits {
			return nil, "", invalidKey("RSA keys shorter than 2048 bits are not accepted")
		}
	}
	return key, comment, nil
}

func writeKeyAudit(ctx context.Context, tx pgx.Tx, action string, p principal.Principal, id uuid.UUID, fingerprint string) error {
	return auditlog.Write(ctx, tx, auditlog.Event{
		OrgID: &p.OrgID, ActorUserID: &p.UserID, ActorKind: "user", Action: action,
		TargetType: "ssh_key", TargetID: id.String(), IP: clientip.From(ctx),
		Data: map[string]any{"fingerprint": fingerprint},
	})
}

// ListSSHKeys returns the caller's keys.
func (s *Service) ListSSHKeys(ctx context.Context) ([]SSHKey, error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return nil, err
	}
	var keys []SSHKey
	err = dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		rows, err := newStore(tx).ListSSHKeys(ctx, p.UserID)
		for _, r := range rows {
			keys = append(keys, keyFromRow(r))
		}
		return err
	})
	return keys, err
}

// AddSSHKey stores publicKey for the caller. An empty name becomes the key's
// comment or "key"; a key already stored by anyone is ErrAlreadyExists.
func (s *Service) AddSSHKey(ctx context.Context, name, publicKey string) (SSHKey, error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return SSHKey{}, err
	}
	key, comment, err := parseSSHKey(publicKey)
	if err != nil {
		return SSHKey{}, err
	}
	if strings.TrimSpace(name) == "" {
		// A comment that is empty or too long falls back to "key".
		name = cmp.Or(strings.TrimSpace(comment), "key")
		if utf8.RuneCountInString(name) > maxKeyName {
			name = "key"
		}
	}
	if name, err = keyName(name); err != nil {
		return SSHKey{}, err
	}
	var k SSHKey
	err = dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		r, err := newStore(tx).CreateSSHKey(ctx, dbq.CreateSSHKeyParams{
			UserID: p.UserID, Name: name,
			PublicKey:         strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))),
			FingerprintSha256: ssh.FingerprintSHA256(key),
		})
		if err != nil {
			return err
		}
		k = keyFromRow(dbq.ListSSHKeysRow(r))
		return writeKeyAudit(ctx, tx, "ssh_key.add", p, k.ID, k.FingerprintSHA256)
	})
	if isUniqueViolation(err) {
		return SSHKey{}, fmt.Errorf("ssh key already registered: %w", apperr.ErrAlreadyExists)
	}
	return k, err
}

// UpdateSSHKey renames the caller's key; another user's key is ErrNotFound.
func (s *Service) UpdateSSHKey(ctx context.Context, id uuid.UUID, name string) (SSHKey, error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return SSHKey{}, err
	}
	if name, err = keyName(name); err != nil {
		return SSHKey{}, err
	}
	var k SSHKey
	err = dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		r, err := newStore(tx).UpdateSSHKey(ctx, dbq.UpdateSSHKeyParams{ID: id, UserID: p.UserID, Name: name})
		if err != nil {
			return err
		}
		k = keyFromRow(dbq.ListSSHKeysRow(r))
		return writeKeyAudit(ctx, tx, "ssh_key.update", p, id, k.FingerprintSHA256)
	})
	return k, keyNotFound(err)
}

// DeleteSSHKey removes the caller's key; another user's key is ErrNotFound.
func (s *Service) DeleteSSHKey(ctx context.Context, id uuid.UUID) error {
	p, err := principal.Require(ctx)
	if err != nil {
		return err
	}
	return keyNotFound(dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		fp, err := newStore(tx).DeleteSSHKey(ctx, dbq.DeleteSSHKeyParams{ID: id, UserID: p.UserID})
		if err != nil {
			return err
		}
		return writeKeyAudit(ctx, tx, "ssh_key.delete", p, id, fp)
	}))
}

func keyNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("ssh key not found: %w", apperr.ErrNotFound)
	}
	return err
}
