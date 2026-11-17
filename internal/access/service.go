package access

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/internal/access/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// Service manages the access bindings of an organization.
type Service struct{ pool *pgxpool.Pool }

// New creates a Service.
func New(pool *pgxpool.Pool) *Service { return &Service{pool} }

func (s *Service) tx(ctx context.Context, orgID, userID uuid.UUID, fn func(Store, pgx.Tx) error) error {
	return dbx.InOrg(ctx, s.pool, orgID, userID, func(tx pgx.Tx) error { return fn(newStore(tx), tx) })
}

func audit(ctx context.Context, tx pgx.Tx, orgID, actor uuid.UUID, action string, id uuid.UUID, data map[string]any) error {
	return auditlog.Write(ctx, tx, auditlog.Event{OrgID: &orgID, ActorUserID: &actor, ActorKind: "user",
		Action: action, TargetType: "access_binding", TargetID: id.String(), IP: clientip.From(ctx), Data: data})
}

// Create adds a binding. The subject must belong to orgID, the resource too.
func (s *Service) Create(ctx context.Context, orgID uuid.UUID, in CreateInput) (Binding, error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return Binding{}, err
	}
	if err := in.validate(); err != nil {
		return Binding{}, err
	}
	arg := dbq.CreateBindingParams{OrgID: orgID, SubjectType: in.SubjectType, SubjectID: in.SubjectID,
		ResourceID: in.ResourceID, CreatedBy: p.UserID}
	arg.Ports, arg.EndPorts = portArrays(in.Ports)
	if len(in.LabelSelector) > 0 {
		if arg.LabelSelector, err = json.Marshal(in.LabelSelector); err != nil {
			return Binding{}, err
		}
	}
	var out Binding
	err = s.tx(ctx, orgID, p.UserID, func(st Store, tx pgx.Tx) error {
		row, err := st.CreateBinding(ctx, arg)
		if err != nil {
			return err
		}
		if out, err = toBinding(dbq.ListBindingsRow(row)); err != nil {
			return err
		}
		data := map[string]any{"subject_type": in.SubjectType, "subject_id": in.SubjectID,
			"self_grant": in.SubjectType == "user" && in.SubjectID == p.UserID}
		if in.ResourceID != nil {
			data["resource_id"] = in.ResourceID
		} else {
			data["label_selector"] = in.LabelSelector
		}
		if len(in.Ports) > 0 {
			data["ports"] = in.Ports
		}
		return audit(ctx, tx, orgID, p.UserID, "access.binding.create", out.ID, data)
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows): // the guarded INSERT matched no subject
		return Binding{}, invalid("subject is not in organization")
	case isForeignKeyViolation(err): // the resource is not in the organization
		return Binding{}, fmt.Errorf("resource not found: %w", apperr.ErrNotFound)
	}
	return out, err
}

// Delete removes a binding.
func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	p, err := principal.Require(ctx)
	if err != nil {
		return err
	}
	return s.tx(ctx, orgID, p.UserID, func(st Store, tx pgx.Tx) error {
		row, err := st.DeleteBinding(ctx, dbq.DeleteBindingParams{ID: id, OrgID: orgID})
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("binding not found: %w", apperr.ErrNotFound)
		}
		if err != nil {
			return err
		}
		data := map[string]any{"subject_type": row.SubjectType, "subject_id": row.SubjectID}
		if row.ResourceID != nil {
			data["resource_id"] = row.ResourceID
		} else {
			data["label_selector"] = json.RawMessage(row.LabelSelector)
		}
		return audit(ctx, tx, orgID, p.UserID, "access.binding.delete", id, data)
	})
}

// List returns the bindings of the organization; with resourceID only those to
// that resource plus every selector binding. It needs no principal: the policy
// snapshot reads it from the gateway too.
func (s *Service) List(ctx context.Context, orgID uuid.UUID, resourceID *uuid.UUID) ([]Binding, error) {
	p, _ := principal.From(ctx)
	var out []Binding
	err := s.tx(ctx, orgID, p.UserID, func(st Store, _ pgx.Tx) error {
		rows, err := st.ListBindings(ctx, dbq.ListBindingsParams{OrgID: orgID, ResourceID: resourceID})
		if err != nil {
			return err
		}
		for _, r := range rows {
			b, err := toBinding(r)
			if err != nil {
				return err
			}
			out = append(out, b)
		}
		return nil
	})
	return out, err
}
