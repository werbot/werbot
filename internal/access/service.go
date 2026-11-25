package access

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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

// Config configures a Service.
type Config struct {
	WebURL string // base of links in emails: <WebURL>/requests?id=<id>
}

// EmailQueue queues an email inside the caller's transaction.
type EmailQueue interface {
	EnqueueTx(ctx context.Context, tx pgx.Tx, to, template string, data map[string]string) error
}

// Settings are the organization settings of approvals and grants.
type Settings struct {
	GrantDefaultTTL, GrantMaxTTL, RequestPendingTTL, CommandApprovalTTL time.Duration
}

// SettingsSource reads the Settings of an organization.
type SettingsSource interface {
	AccessSettings(ctx context.Context, orgID uuid.UUID) (Settings, error)
}

// Service manages the access bindings, approvals and grants of an organization.
type Service struct {
	pool     *pgxpool.Pool
	emails   EmailQueue
	settings SettingsSource
	cfg      Config
}

// New creates a Service. Bindings need neither emails nor settings (nil is fine there).
func New(pool *pgxpool.Pool, emails EmailQueue, settings SettingsSource, cfg Config) *Service {
	return &Service{pool: pool, emails: emails, settings: settings, cfg: cfg}
}

func (s *Service) tx(ctx context.Context, orgID, userID uuid.UUID, fn func(Store, pgx.Tx) error) error {
	return dbx.InOrg(ctx, s.pool, orgID, userID, func(tx pgx.Tx) error { return fn(newStore(tx), tx) })
}

func audit(ctx context.Context, tx pgx.Tx, orgID, actor uuid.UUID, action, targetType string, id uuid.UUID, data map[string]any) error {
	return auditAs(ctx, tx, orgID, "user", &actor, action, targetType, id, data)
}

// auditAs is audit by an actor of kind ("user", "gateway" or "system"); actor may be nil.
func auditAs(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, kind string, actor *uuid.UUID, action, targetType string, id uuid.UUID, data map[string]any) error {
	return auditlog.Write(ctx, tx, auditlog.Event{OrgID: &orgID, ActorUserID: actor, ActorKind: kind,
		Action: action, TargetType: targetType, TargetID: id.String(), IP: clientip.From(ctx), Data: data})
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
		if len(out.Ports) > 0 {
			data["ports"] = out.Ports
		}
		return audit(ctx, tx, orgID, p.UserID, "access.binding.create", "access_binding", out.ID, data)
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
		ports, err := portRanges(row.Ports, row.EndPorts)
		if err != nil {
			return fmt.Errorf("access: ports of %s: %w", id, err)
		}
		if len(ports) > 0 {
			data["ports"] = ports
		}
		return audit(ctx, tx, orgID, p.UserID, "access.binding.delete", "access_binding", id, data)
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
