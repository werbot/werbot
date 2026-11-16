package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/inventory/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// Service manages the resources of an organization.
type Service struct{ pool *pgxpool.Pool }

// New creates a Service.
func New(pool *pgxpool.Pool) *Service { return &Service{pool} }

// tx runs fn in orgID as the caller (uuid.Nil without a principal).
func (s *Service) tx(ctx context.Context, orgID uuid.UUID, fn func(Store, pgx.Tx) error) error {
	p, _ := principal.From(ctx)
	return dbx.InOrg(ctx, s.pool, orgID, p.UserID, func(tx pgx.Tx) error { return fn(newStore(tx), tx) })
}

func audit(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, action string, id uuid.UUID, data map[string]any) error {
	e := auditlog.Event{OrgID: &orgID, ActorKind: "system", Action: action,
		TargetType: "resource", TargetID: id.String(), IP: clientip.From(ctx), Data: data}
	if p, ok := principal.From(ctx); ok {
		e.ActorUserID, e.ActorKind = &p.UserID, "user"
	}
	return auditlog.Write(ctx, tx, e)
}

// notFound maps no rows and a foreign-key miss (resource of another org) to ErrNotFound,
// a duplicate name to ErrAlreadyExists.
func notFound(err error) error {
	switch {
	case errors.Is(err, pgx.ErrNoRows), isForeignKeyViolation(err):
		return fmt.Errorf("resource not found: %w", apperr.ErrNotFound)
	case isUniqueViolation(err):
		return fmt.Errorf("resource name already used: %w", apperr.ErrAlreadyExists)
	}
	return err
}

func (s *Service) get(ctx context.Context, st Store, orgID, id uuid.UUID) (Resource, error) {
	r, err := st.GetResource(ctx, dbq.GetResourceParams{ID: id, OrgID: orgID})
	if err != nil {
		return Resource{}, err
	}
	return toResource(r)
}

// labelsJSON encodes m as a JSON object; a nil map is {}, not null.
func labelsJSON(m map[string]string) ([]byte, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(m)
}

func setLabels(ctx context.Context, st Store, orgID, id uuid.UUID, labels map[string]string) error {
	if err := st.DeleteLabels(ctx, dbq.DeleteLabelsParams{ResourceID: id, OrgID: orgID}); err != nil {
		return err
	}
	raw, err := labelsJSON(labels)
	if err != nil {
		return err
	}
	return st.InsertLabels(ctx, dbq.InsertLabelsParams{ResourceID: id, OrgID: orgID, Labels: raw})
}

// Create adds a resource of a registered type; Config is validated by the type.
func (s *Service) Create(ctx context.Context, orgID uuid.UUID, in CreateInput) (Resource, error) {
	t, ok := connector.Lookup(in.Type)
	if !ok {
		return Resource{}, invalid("unknown resource type %q", in.Type)
	}
	if err := errors.Join(validateName(in.Name), validateLabels(in.Labels)); err != nil {
		return Resource{}, err
	}
	cfg, err := validateConfig(t, in.Config)
	if err != nil {
		return Resource{}, err
	}
	var res Resource
	err = s.tx(ctx, orgID, func(st Store, tx pgx.Tx) error {
		id, err := st.CreateResource(ctx, dbq.CreateResourceParams{
			OrgID: orgID, Name: in.Name, Type: t.Name, Category: string(t.Category), GatewayID: in.GatewayID, Config: cfg})
		if err != nil {
			return err
		}
		if err := setLabels(ctx, st, orgID, id, in.Labels); err != nil {
			return err
		}
		if res, err = s.get(ctx, st, orgID, id); err != nil {
			return err
		}
		return audit(ctx, tx, orgID, "resource.create", id, map[string]any{"name": in.Name, "type": t.Name})
	})
	return res, notFound(err)
}

// Update replaces name, gateway and labels; a non-nil Config replaces the config.
func (s *Service) Update(ctx context.Context, orgID uuid.UUID, in UpdateInput) (Resource, error) {
	if err := errors.Join(validateName(in.Name), validateLabels(in.Labels)); err != nil {
		return Resource{}, err
	}
	var res Resource
	err := s.tx(ctx, orgID, func(st Store, tx pgx.Tx) error {
		cur, err := s.get(ctx, st, orgID, in.ID)
		if err != nil {
			return err
		}
		var cfg []byte
		if in.Config != nil {
			t, ok := connector.Lookup(cur.Type)
			if !ok {
				return invalid("resource type %q is no longer registered", cur.Type)
			}
			if cfg, err = validateConfig(t, in.Config); err != nil {
				return err
			}
		}
		if _, err := st.UpdateResource(ctx, dbq.UpdateResourceParams{
			ID: in.ID, OrgID: orgID, Name: in.Name, GatewayID: in.GatewayID, Config: cfg}); err != nil {
			return err
		}
		if err := setLabels(ctx, st, orgID, in.ID, in.Labels); err != nil {
			return err
		}
		if res, err = s.get(ctx, st, orgID, in.ID); err != nil {
			return err
		}
		return audit(ctx, tx, orgID, "resource.update", in.ID, map[string]any{"name": in.Name})
	})
	return res, notFound(err)
}

// Delete removes a resource with everything that hangs on it.
func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return notFound(s.tx(ctx, orgID, func(st Store, tx pgx.Tx) error {
		n, err := st.DeleteResource(ctx, dbq.DeleteResourceParams{ID: id, OrgID: orgID})
		if err != nil {
			return err
		}
		if n == 0 {
			return pgx.ErrNoRows
		}
		return audit(ctx, tx, orgID, "resource.delete", id, nil)
	}))
}

// Get returns one resource of the organization.
func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (Resource, error) {
	var res Resource
	err := s.tx(ctx, orgID, func(st Store, _ pgx.Tx) (err error) {
		res, err = s.get(ctx, st, orgID, id)
		return err
	})
	return res, notFound(err)
}

func (s *Service) list(ctx context.Context, orgID uuid.UUID, gatewayID *uuid.UUID, filter map[string]string) ([]Resource, error) {
	raw, err := labelsJSON(filter)
	if err != nil {
		return nil, err
	}
	var out []Resource
	err = s.tx(ctx, orgID, func(st Store, _ pgx.Tx) error {
		rows, err := st.ListResources(ctx, dbq.ListResourcesParams{OrgID: orgID, GatewayID: gatewayID, LabelFilter: raw})
		for _, r := range rows {
			res, cerr := toResource(dbq.GetResourceRow(r))
			if cerr != nil {
				return cerr
			}
			out = append(out, res)
		}
		return err
	})
	return out, err
}

// List returns resources by name, only those carrying every label of labelFilter.
func (s *Service) List(ctx context.Context, orgID uuid.UUID, labelFilter map[string]string) ([]Resource, error) {
	return s.list(ctx, orgID, nil, labelFilter)
}

// ResourcesForGateway returns the resources served by gatewayID.
func (s *Service) ResourcesForGateway(ctx context.Context, orgID, gatewayID uuid.UUID) ([]Resource, error) {
	return s.list(ctx, orgID, &gatewayID, nil)
}
