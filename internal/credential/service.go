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

// audit records action; data holds ids, slot and kind only, never the secret.
func audit(ctx context.Context, tx pgx.Tx, orgID, resourceID uuid.UUID, action, slot string, kind connector.CredentialKind) error {
	e := auditlog.Event{OrgID: &orgID, ActorKind: "system", Action: action, TargetType: "resource",
		TargetID: resourceID.String(), IP: clientip.From(ctx),
		Data: map[string]any{"resource_id": resourceID.String(), "slot": slot, "kind": string(kind)}}
	if p, ok := principal.From(ctx); ok {
		e.ActorUserID, e.ActorKind = &p.UserID, "user"
	}
	return auditlog.Write(ctx, tx, e)
}

// markAgentReadonly sets the verification status of the agent_readonly slot
// (pending after Set, none after Clear); the primary slot has none.
func markAgentReadonly(ctx context.Context, st Store, orgID, resourceID uuid.UUID, slot, status string) error {
	if slot != SlotAgentReadonly {
		return nil
	}
	return st.SetAgentReadonlyStatus(ctx, dbq.SetAgentReadonlyStatusParams{Status: status, ResourceID: resourceID, OrgID: orgID})
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) || isForeignKeyViolation(err) {
		return fmt.Errorf("credential not found: %w", apperr.ErrNotFound)
	}
	return err
}

// Set replaces the credential of a resource in slot, sealing value with the
// current key. The agent_readonly slot marks the resource pending in the same
// transaction.
func (s *Service) Set(ctx context.Context, orgID, resourceID uuid.UUID, slot string, kind connector.CredentialKind, value []byte) error {
	if err := checkSlot(slot); err != nil {
		return err
	}
	if err := validate(kind, value); err != nil {
		return err
	}
	sealed, keyID, err := s.box.Seal(value)
	if err != nil {
		return fmt.Errorf("credential: seal: %w", err)
	}
	return notFound(s.tx(ctx, orgID, func(st Store, tx pgx.Tx) error {
		n, err := st.UpsertCredential(ctx, dbq.UpsertCredentialParams{
			OrgID: orgID, ResourceID: resourceID, Kind: string(kind), Ciphertext: sealed, KeyID: keyID, Slot: slot})
		if err != nil {
			return err
		}
		if n == 0 {
			return pgx.ErrNoRows
		}
		if err := markAgentReadonly(ctx, st, orgID, resourceID, slot, "pending"); err != nil {
			return err
		}
		return audit(ctx, tx, orgID, resourceID, "credential.set", slot, kind)
	}))
}

// Clear removes the credential of a resource in slot; none stored is not an
// error. The agent_readonly slot marks the resource none in the same transaction.
func (s *Service) Clear(ctx context.Context, orgID, resourceID uuid.UUID, slot string) error {
	if err := checkSlot(slot); err != nil {
		return err
	}
	return s.tx(ctx, orgID, func(st Store, tx pgx.Tx) error {
		key := dbq.GetCredentialParams{ResourceID: resourceID, OrgID: orgID, Slot: slot}
		cur, err := st.GetCredential(ctx, key)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if _, err := st.DeleteCredential(ctx, dbq.DeleteCredentialParams(key)); err != nil {
			return err
		}
		if err := markAgentReadonly(ctx, st, orgID, resourceID, slot, "none"); err != nil {
			return err
		}
		return audit(ctx, tx, orgID, resourceID, "credential.clear", slot, connector.CredentialKind(cur.Kind))
	})
}

// Kinds returns the stored kind of slot per resource; a resource without a
// credential in slot has no key.
func (s *Service) Kinds(ctx context.Context, orgID uuid.UUID, resourceIDs []uuid.UUID, slot string) (map[uuid.UUID]connector.CredentialKind, error) {
	if err := checkSlot(slot); err != nil {
		return nil, err
	}
	out := map[uuid.UUID]connector.CredentialKind{}
	err := s.tx(ctx, orgID, func(st Store, _ pgx.Tx) error {
		rows, err := st.ListCredentialKinds(ctx, dbq.ListCredentialKindsParams{OrgID: orgID, Slot: slot, ResourceIds: resourceIDs})
		for _, r := range rows {
			out[r.ResourceID] = connector.CredentialKind(r.Kind)
		}
		return err
	})
	return out, err
}

// Reveal decrypts the credential in slot. Only the gateway paths may call it.
func (s *Service) Reveal(ctx context.Context, orgID, resourceID uuid.UUID, slot string) (Secret, error) {
	if err := checkSlot(slot); err != nil {
		return Secret{}, err
	}
	var row dbq.GetCredentialRow
	p, _ := principal.From(ctx)
	err := dbx.ReadInOrg(s.pool, orgID, p.UserID, func(r *dbx.Read) (err error) {
		row, err = newStore(r).GetCredential(ctx, dbq.GetCredentialParams{ResourceID: resourceID, OrgID: orgID, Slot: slot})
		return err
	})
	if err != nil {
		return Secret{}, notFound(err)
	}
	value, err := s.box.Open(row.Ciphertext, row.KeyID)
	if err != nil {
		return Secret{}, fmt.Errorf("credential: open: %w", err)
	}
	return Secret{Kind: connector.CredentialKind(row.Kind), Value: value, Version: row.UpdatedAt.UnixMicro()}, nil
}
