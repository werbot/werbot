package access

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/access/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// LockInput describes a new lock; ExpiresIn 0 locks until the lock is deleted.
type LockInput struct {
	TargetType string
	TargetID   *uuid.UUID
	Reason     string
	ExpiresIn  time.Duration
}

// LockView is a lock with who created it and when.
type LockView struct {
	Lock
	CreatedBy *uuid.UUID
	CreatedAt time.Time
}

// The lock target types are also listed in CreateLockRequest.target_type
// (proto/werbot/v1/lock.proto) and in writeLocks (internal/policy/compile_jit.go): change them together.
var (
	// lockTargets are the target types a lock takes now.
	lockTargets = []string{"user", "team", "resource", "grant", "cli_session"}
	// laterLockTargets come with agents (stage 12) and MFA (stage 14).
	laterLockTargets = []string{"agent", "agent_run", "agents_of_user", "all_agents", "mfa_device"}
)

func (in LockInput) validate() (string, error) {
	switch {
	case slices.Contains(laterLockTargets, in.TargetType):
		return "", invalid("target type %s is not supported yet", in.TargetType)
	case !slices.Contains(lockTargets, in.TargetType):
		return "", invalid("unknown target type")
	case in.TargetID == nil:
		return "", invalid("target id is required")
	case in.ExpiresIn < 0:
		return "", invalid("expires in must not be negative")
	}
	reason, err := text("reason", in.Reason)
	if err == nil && reason == "" {
		err = invalid("reason is required")
	}
	return reason, err
}

func toLock(r dbq.Lock) LockView {
	return LockView{Lock: Lock{ID: r.ID, OrgID: r.OrgID, TargetType: r.TargetType, TargetID: r.TargetID, Reason: r.Reason, ExpiresAt: r.ExpiresAt},
		CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt}
}

func requireManager(p principal.Principal) error {
	if !isManager(p.Role) {
		return denied("only owner and admin manage locks")
	}
	return nil
}

// ActiveLocks returns the unexpired locks of orgID by id, for the policy snapshot.
func (s *Service) ActiveLocks(ctx context.Context, orgID uuid.UUID) ([]Lock, error) {
	var out []Lock
	err := s.tx(ctx, orgID, uuid.Nil, func(st Store, _ pgx.Tx) error {
		rows, err := st.ActiveLocks(ctx, orgID)
		for _, r := range rows {
			out = append(out, toLock(r).Lock)
		}
		return err
	})
	return out, err
}

// ListLocks returns the unexpired locks of the organization of p, newest first.
func (s *Service) ListLocks(ctx context.Context, p principal.Principal) ([]LockView, error) {
	if err := requireManager(p); err != nil {
		return nil, err
	}
	var out []LockView
	err := s.tx(ctx, p.OrgID, p.UserID, func(st Store, _ pgx.Tx) error {
		rows, err := st.ListLocks(ctx, p.OrgID)
		for _, r := range rows {
			out = append(out, toLock(r))
		}
		return err
	})
	return out, err
}

// CreateLock denies access to a target of the organization of p; a target of
// another organization is not found.
func (s *Service) CreateLock(ctx context.Context, p principal.Principal, in LockInput) (Lock, error) {
	if err := requireManager(p); err != nil {
		return Lock{}, err
	}
	reason, err := in.validate()
	if err != nil {
		return Lock{}, err
	}
	var out Lock
	err = s.tx(ctx, p.OrgID, p.UserID, func(st Store, tx pgx.Tx) error {
		row, err := st.CreateLock(ctx, dbq.CreateLockParams{OrgID: p.OrgID, TargetType: in.TargetType, TargetID: *in.TargetID,
			Reason: reason, ExpiresSecs: in.ExpiresIn.Seconds(), CreatedBy: p.UserID})
		if errors.Is(err, pgx.ErrNoRows) { // the guarded INSERT matched no target
			return fmt.Errorf("lock target not found: %w", apperr.ErrNotFound)
		}
		if err != nil {
			return err
		}
		out = toLock(row).Lock
		return audit(ctx, tx, p.OrgID, p.UserID, "lock.create", "lock", out.ID,
			map[string]any{"target_type": out.TargetType, "target_id": out.TargetID})
	})
	if err != nil {
		return Lock{}, err
	}
	return out, nil
}

// DeleteLock removes lock id of the organization of p.
func (s *Service) DeleteLock(ctx context.Context, p principal.Principal, id uuid.UUID) error {
	if err := requireManager(p); err != nil {
		return err
	}
	return s.tx(ctx, p.OrgID, p.UserID, func(st Store, tx pgx.Tx) error {
		row, err := st.DeleteLock(ctx, dbq.DeleteLockParams{ID: id, OrgID: p.OrgID})
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("lock not found: %w", apperr.ErrNotFound)
		}
		if err != nil {
			return err
		}
		return audit(ctx, tx, p.OrgID, p.UserID, "lock.delete", "lock", id,
			map[string]any{"target_type": row.TargetType, "target_id": row.TargetID})
	})
}
