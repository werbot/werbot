package inventory

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/inventory/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
)

// PinHostKey pins key on first use (TOFU). Pinning the same key again is a no-op;
// a different key is ErrFailedPrecondition until ClearHostKey. A new pin and a
// clear notify the organization's gateways: their snapshots carry the pin.
func (s *Service) PinHostKey(ctx context.Context, orgID, resourceID uuid.UUID, key ssh.PublicKey) error {
	fp := ssh.FingerprintSHA256(key)
	return notFound(s.tx(ctx, orgID, func(st Store, tx pgx.Tx) error {
		n, err := st.InsertHostKey(ctx, dbq.InsertHostKeyParams{
			ResourceID: resourceID, OrgID: orgID, KeyType: key.Type(),
			PublicKey: string(ssh.MarshalAuthorizedKey(key)), FingerprintSha256: fp})
		if err != nil {
			return err
		}
		if n == 1 {
			if err := st.NotifyHostKeyChanged(ctx, orgID); err != nil {
				return err
			}
			return audit(ctx, tx, orgID, "resource.host_key.pin", resourceID, map[string]any{"fingerprint": fp})
		}
		pinned, err := st.GetHostKeyFingerprint(ctx, dbq.GetHostKeyFingerprintParams{ResourceID: resourceID, OrgID: orgID})
		if err != nil {
			return err
		}
		if pinned != fp {
			return fmt.Errorf("%w: host key differs from the pinned one", apperr.ErrFailedPrecondition)
		}
		return nil
	}))
}

// ClearHostKey forgets the pinned key so the next connection pins anew.
func (s *Service) ClearHostKey(ctx context.Context, orgID, resourceID uuid.UUID) error {
	return notFound(s.tx(ctx, orgID, func(st Store, tx pgx.Tx) error {
		if _, err := s.get(ctx, st, orgID, resourceID); err != nil {
			return err
		}
		n, err := st.DeleteHostKey(ctx, dbq.DeleteHostKeyParams{ResourceID: resourceID, OrgID: orgID})
		if err != nil || n == 0 {
			return err
		}
		if err := st.NotifyHostKeyChanged(ctx, orgID); err != nil {
			return err
		}
		return audit(ctx, tx, orgID, "resource.host_key.clear", resourceID, nil)
	}))
}
