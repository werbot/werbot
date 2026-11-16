package credential

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/credential/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/platform/secretbox"
)

// Service keeps the credentials of an organization's resources.
type Service struct {
	pool *pgxpool.Pool
	box  *secretbox.Box
}

// New creates a Service sealing with box.
func New(pool *pgxpool.Pool, box *secretbox.Box) *Service { return &Service{pool, box} }

// tx runs fn in orgID as the caller (uuid.Nil without a principal).
func (s *Service) tx(ctx context.Context, orgID uuid.UUID, fn func(Store, pgx.Tx) error) error {
	p, _ := principal.From(ctx)
	return dbx.InOrg(ctx, s.pool, orgID, p.UserID, func(tx pgx.Tx) error { return fn(newStore(tx), tx) })
}

// audit records action; data holds ids and kind only, never the secret.
func audit(ctx context.Context, tx pgx.Tx, orgID, resourceID uuid.UUID, action string, kind connector.CredentialKind) error {
	e := auditlog.Event{OrgID: &orgID, ActorKind: "system", Action: action, TargetType: "resource",
		TargetID: resourceID.String(), IP: clientip.From(ctx),
		Data: map[string]any{"resource_id": resourceID.String(), "kind": string(kind)}}
	if p, ok := principal.From(ctx); ok {
		e.ActorUserID, e.ActorKind = &p.UserID, "user"
	}
	return auditlog.Write(ctx, tx, e)
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) || isForeignKeyViolation(err) {
		return fmt.Errorf("credential not found: %w", apperr.ErrNotFound)
	}
	return err
}

// Set replaces the credential of a resource, sealing value with the current key.
func (s *Service) Set(ctx context.Context, orgID, resourceID uuid.UUID, kind connector.CredentialKind, value []byte) error {
	if err := validate(kind, value); err != nil {
		return err
	}
	sealed, keyID, err := s.box.Seal(value)
	if err != nil {
		return fmt.Errorf("credential: seal: %w", err)
	}
	return notFound(s.tx(ctx, orgID, func(st Store, tx pgx.Tx) error {
		n, err := st.UpsertCredential(ctx, dbq.UpsertCredentialParams{
			OrgID: orgID, ResourceID: resourceID, Kind: string(kind), Ciphertext: sealed, KeyID: keyID})
		if err != nil {
			return err
		}
		if n == 0 {
			return pgx.ErrNoRows
		}
		return audit(ctx, tx, orgID, resourceID, "credential.set", kind)
	}))
}

// Clear removes the credential of a resource; none stored is not an error.
func (s *Service) Clear(ctx context.Context, orgID, resourceID uuid.UUID) error {
	return s.tx(ctx, orgID, func(st Store, tx pgx.Tx) error {
		cur, err := st.GetCredential(ctx, dbq.GetCredentialParams{ResourceID: resourceID, OrgID: orgID})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if _, err := st.DeleteCredential(ctx, dbq.DeleteCredentialParams{ResourceID: resourceID, OrgID: orgID}); err != nil {
			return err
		}
		return audit(ctx, tx, orgID, resourceID, "credential.clear", connector.CredentialKind(cur.Kind))
	})
}

// Kinds returns the stored kind per resource; a resource without credential has no key.
func (s *Service) Kinds(ctx context.Context, orgID uuid.UUID, resourceIDs []uuid.UUID) (map[uuid.UUID]connector.CredentialKind, error) {
	out := map[uuid.UUID]connector.CredentialKind{}
	err := s.tx(ctx, orgID, func(st Store, _ pgx.Tx) error {
		rows, err := st.ListCredentialKinds(ctx, dbq.ListCredentialKindsParams{OrgID: orgID, ResourceIds: resourceIDs})
		for _, r := range rows {
			out[r.ResourceID] = connector.CredentialKind(r.Kind)
		}
		return err
	})
	return out, err
}

// Reveal decrypts the credential. Only the gateway session path may call it.
func (s *Service) Reveal(ctx context.Context, orgID, resourceID uuid.UUID) (Secret, error) {
	var row dbq.GetCredentialRow
	err := s.tx(ctx, orgID, func(st Store, _ pgx.Tx) (err error) {
		row, err = st.GetCredential(ctx, dbq.GetCredentialParams{ResourceID: resourceID, OrgID: orgID})
		return err
	})
	if err != nil {
		return Secret{}, notFound(err)
	}
	value, err := s.box.Open(row.Ciphertext, row.KeyID)
	if err != nil {
		return Secret{}, fmt.Errorf("credential: open: %w", err)
	}
	return Secret{Kind: connector.CredentialKind(row.Kind), Value: value}, nil
}
