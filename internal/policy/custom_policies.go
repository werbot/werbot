package policy

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/policy/dbq"
)

// CustomPolicyInput describes a custom Cedar policy text.
type CustomPolicyInput struct {
	Name, Cedar string
	Enabled     bool
}

// CustomPolicy is a stored custom policy.
type CustomPolicy struct {
	ID uuid.UUID
	CustomPolicyInput
	CreatedBy *uuid.UUID // nil: the creator was deleted
	UpdatedAt time.Time
}

// validate checks the name and the Cedar text; a Cedar error keeps its full
// text with the position.
func (in CustomPolicyInput) validate() error {
	if err := validName(in.Name); err != nil {
		return err
	}
	if err := ValidateCustom(in.Cedar); err != nil {
		return fmt.Errorf("%w: %w", apperr.ErrInvalidArgument, err)
	}
	return nil
}

func toCustomPolicy(r dbq.CustomPolicy) CustomPolicy {
	return CustomPolicy{ID: r.ID, CustomPolicyInput: CustomPolicyInput{Name: r.Name, Cedar: r.Cedar, Enabled: r.Enabled},
		CreatedBy: r.CreatedBy, UpdatedAt: r.UpdatedAt}
}

// customPolicyChange audits the name and the state; the Cedar text stays out.
func customPolicyChange(action string, r dbq.CustomPolicy) change {
	return change{Action: action, ID: r.ID, Data: map[string]any{"name": r.Name, "enabled": r.Enabled}}
}

// ListCustomPolicies returns the custom policies of orgID by name.
func (s *Service) ListCustomPolicies(ctx context.Context, orgID uuid.UUID) ([]CustomPolicy, error) {
	var out []CustomPolicy
	err := dbx.InOrg(ctx, s.deps.Pool, orgID, actor(ctx), func(tx pgx.Tx) error {
		rows, err := newStore(tx).ListCustomPolicies(ctx, orgID)
		for _, r := range rows {
			out = append(out, toCustomPolicy(r))
		}
		return err
	})
	return out, err
}

// CreateCustomPolicy adds a custom policy to orgID; a permit in it widens access.
func (s *Service) CreateCustomPolicy(ctx context.Context, orgID uuid.UUID, in CustomPolicyInput) (CustomPolicy, error) {
	return s.writeCustomPolicy(ctx, orgID, in, "custom_policy.create", func(st Store, actor uuid.UUID) (dbq.CustomPolicy, error) {
		return st.CreateCustomPolicy(ctx, dbq.CreateCustomPolicyParams{OrgID: orgID, Name: in.Name, Cedar: in.Cedar, Enabled: in.Enabled,
			CreatedBy: &actor})
	})
}

// UpdateCustomPolicy replaces the custom policy id as a whole.
func (s *Service) UpdateCustomPolicy(ctx context.Context, orgID, id uuid.UUID, in CustomPolicyInput) (CustomPolicy, error) {
	return s.writeCustomPolicy(ctx, orgID, in, "custom_policy.update", func(st Store, _ uuid.UUID) (dbq.CustomPolicy, error) {
		return st.UpdateCustomPolicy(ctx, dbq.UpdateCustomPolicyParams{OrgID: orgID, ID: id, Name: in.Name, Cedar: in.Cedar, Enabled: in.Enabled})
	})
}

// writeCustomPolicy validates in and runs write in one audited transaction.
func (s *Service) writeCustomPolicy(ctx context.Context, orgID uuid.UUID, in CustomPolicyInput, action string,
	write func(Store, uuid.UUID) (dbq.CustomPolicy, error)) (CustomPolicy, error) {
	if err := in.validate(); err != nil {
		return CustomPolicy{}, err
	}
	var out CustomPolicy
	err := s.apply(ctx, orgID, "custom policy", func(st Store, actor uuid.UUID) (change, error) {
		row, err := write(st, actor)
		out = toCustomPolicy(row)
		return customPolicyChange(action, row), err
	})
	return out, err
}

// DeleteCustomPolicy removes the custom policy id.
func (s *Service) DeleteCustomPolicy(ctx context.Context, orgID, id uuid.UUID) error {
	return s.apply(ctx, orgID, "custom policy", func(st Store, _ uuid.UUID) (change, error) {
		row, err := st.DeleteCustomPolicy(ctx, dbq.DeleteCustomPolicyParams{OrgID: orgID, ID: id})
		return customPolicyChange("custom_policy.delete", row), err
	})
}
