package policy

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// change is the audit event of a write: Action is "<target type>.<verb>".
type change struct {
	Action string
	ID     uuid.UUID
	Data   map[string]any
}

// apply runs fn and writes the audit event it returns in one transaction of
// the principal. what names the object in not-found and taken errors.
func (s *Service) apply(ctx context.Context, orgID uuid.UUID, what string, fn func(st Store, actor uuid.UUID) (change, error)) error {
	p, err := principal.Require(ctx)
	if err != nil {
		return err
	}
	err = dbx.InOrg(ctx, s.deps.Pool, orgID, p.UserID, func(tx pgx.Tx) error {
		c, err := fn(newStore(tx), p.UserID)
		if err != nil {
			return err
		}
		typ, _, _ := strings.Cut(c.Action, ".")
		return auditlog.Write(ctx, tx, auditlog.Event{OrgID: &orgID, ActorUserID: &p.UserID, ActorKind: "user", Action: c.Action,
			TargetType: typ, TargetID: c.ID.String(), IP: clientip.From(ctx), Data: c.Data})
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return fmt.Errorf("%s not found: %w", what, apperr.ErrNotFound)
	case isUniqueViolation(err):
		return fmt.Errorf("%s already exists: %w", what, apperr.ErrAlreadyExists)
	}
	return err
}
