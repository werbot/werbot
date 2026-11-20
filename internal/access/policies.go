package access

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/access/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

const (
	defaultPolicyDuration = 8 * time.Hour
	minPolicyDuration     = time.Minute
	maxPolicyDuration     = 14 * 24 * time.Hour
	maxSelectorResources  = 100
)

// distinct counts the different ids.
func distinct(ids []uuid.UUID) int {
	set := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return len(set)
}

func (s Selector) validate() error {
	if (len(s.ResourceIDs) > 0) == (len(s.Labels) > 0) {
		return invalid("set exactly one of resources and labels in the selector")
	}
	if len(s.ResourceIDs) > maxSelectorResources {
		return invalid("at most %d selector resources", maxSelectorResources)
	}
	if len(s.Labels) > maxSelectorLabels {
		return invalid("at most %d selector labels", maxSelectorLabels)
	}
	for k, v := range s.Labels {
		if !labelKeyRe.MatchString(k) {
			return invalid("label key %q must match %s", k, labelKeyRe)
		}
		if !labelValueRe.MatchString(v) {
			return invalid("label value of %q must match %s", k, labelValueRe)
		}
	}
	return nil
}

// validate checks the policy and fills the default duration; it does not look at the database.
func (p *ApprovalPolicy) validate() error {
	if n := len(p.Name); n < 1 || n > 63 {
		return invalid("name must have 1 to 63 bytes")
	}
	if p.Mode != ModeManual && p.Mode != ModeAuto {
		return invalid("mode must be %s or %s", ModeManual, ModeAuto)
	}
	if p.MaxDuration == 0 {
		p.MaxDuration = defaultPolicyDuration
	}
	if p.MaxDuration < minPolicyDuration || p.MaxDuration > maxPolicyDuration {
		return invalid("max duration must be between %s and %s", minPolicyDuration, maxPolicyDuration)
	}
	if err := p.Selector.validate(); err != nil {
		return err
	}
	r := p.Requesters
	if len(r.Users) == 0 && len(r.Teams) == 0 && !r.AllUsers && !r.AllAgents {
		return invalid("requesters must not be empty")
	}
	if p.Approvers.AllUsers || p.Approvers.AllAgents {
		return invalid("approvers are users and teams only")
	}
	return nil
}

// checkRefs refuses users, teams and resources of another organization.
func (p ApprovalPolicy) checkRefs(ctx context.Context, st Store, orgID uuid.UUID) error {
	users := append(append([]uuid.UUID(nil), p.Requesters.Users...), p.Approvers.Users...)
	teams := append(append([]uuid.UUID(nil), p.Requesters.Teams...), p.Approvers.Teams...)
	n, err := st.CountPolicyRefs(ctx, dbq.CountPolicyRefsParams{OrgID: orgID, ResourceIds: p.Selector.ResourceIDs, UserIds: users, TeamIds: teams})
	if err != nil {
		return err
	}
	if int(n.Users) != distinct(users) || int(n.Teams) != distinct(teams) {
		return invalid("subject is not in organization")
	}
	if int(n.Resources) != distinct(p.Selector.ResourceIDs) {
		return invalid("resource is not in organization")
	}
	return nil
}

func toPolicy(r dbq.ListApprovalPoliciesRow) (ApprovalPolicy, error) {
	p := ApprovalPolicy{ID: r.ID, OrgID: r.OrgID, Name: r.Name, Mode: r.Mode, MaxDuration: time.Duration(r.MaxDurationSecs) * time.Second,
		RequireReason: r.RequireReason, FourEyes: r.FourEyes, BreakGlass: r.BreakGlass, CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt}
	for _, f := range []struct {
		raw []byte
		to  any
	}{{r.ResourceSelector, &p.Selector}, {r.Requesters, &p.Requesters}, {r.Approvers, &p.Approvers}} {
		if err := json.Unmarshal(f.raw, f.to); err != nil {
			return ApprovalPolicy{}, fmt.Errorf("access: decode policy %s: %w", r.ID, err)
		}
	}
	return p, nil
}

// ListApprovalPolicies returns the policies of the organization by name.
func (s *Service) ListApprovalPolicies(ctx context.Context, orgID uuid.UUID) ([]ApprovalPolicy, error) {
	p, _ := principal.From(ctx)
	var out []ApprovalPolicy
	err := s.tx(ctx, orgID, p.UserID, func(st Store, _ pgx.Tx) error {
		rows, err := st.ListApprovalPolicies(ctx, orgID)
		if err != nil {
			return err
		}
		for _, r := range rows {
			pol, err := toPolicy(r)
			if err != nil {
				return err
			}
			out = append(out, pol)
		}
		return nil
	})
	return out, err
}

// CreateApprovalPolicy adds a policy to the organization of p.
func (s *Service) CreateApprovalPolicy(ctx context.Context, p principal.Principal, in ApprovalPolicy) (ApprovalPolicy, error) {
	return s.writePolicy(ctx, p, in, "approval_policy.create", func(st Store, arg dbq.UpdateApprovalPolicyParams) (dbq.ListApprovalPoliciesRow, error) {
		row, err := st.CreateApprovalPolicy(ctx, dbq.CreateApprovalPolicyParams{OrgID: p.OrgID, Name: arg.Name, ResourceSelector: arg.ResourceSelector,
			Requesters: arg.Requesters, Approvers: arg.Approvers, Mode: arg.Mode, MaxDurationSecs: arg.MaxDurationSecs,
			RequireReason: arg.RequireReason, FourEyes: arg.FourEyes, BreakGlass: arg.BreakGlass, CreatedBy: &p.UserID})
		return dbq.ListApprovalPoliciesRow(row), err
	})
}

// UpdateApprovalPolicy replaces the policy in.ID as a whole.
func (s *Service) UpdateApprovalPolicy(ctx context.Context, p principal.Principal, in ApprovalPolicy) (ApprovalPolicy, error) {
	return s.writePolicy(ctx, p, in, "approval_policy.update", func(st Store, arg dbq.UpdateApprovalPolicyParams) (dbq.ListApprovalPoliciesRow, error) {
		arg.ID = in.ID
		row, err := st.UpdateApprovalPolicy(ctx, arg)
		return dbq.ListApprovalPoliciesRow(row), err
	})
}

// writePolicy validates in, runs write and audits it in one transaction.
func (s *Service) writePolicy(ctx context.Context, p principal.Principal, in ApprovalPolicy, action string,
	write func(Store, dbq.UpdateApprovalPolicyParams) (dbq.ListApprovalPoliciesRow, error)) (ApprovalPolicy, error) {
	if err := in.validate(); err != nil {
		return ApprovalPolicy{}, err
	}
	arg := dbq.UpdateApprovalPolicyParams{OrgID: p.OrgID, Name: in.Name, Mode: in.Mode, MaxDurationSecs: in.MaxDuration.Seconds(),
		RequireReason: in.RequireReason, FourEyes: in.FourEyes, BreakGlass: in.BreakGlass}
	var err error
	for _, f := range []struct {
		from any
		to   *[]byte
	}{{in.Selector, &arg.ResourceSelector}, {in.Requesters, &arg.Requesters}, {in.Approvers, &arg.Approvers}} {
		if *f.to, err = json.Marshal(f.from); err != nil {
			return ApprovalPolicy{}, err
		}
	}
	var out ApprovalPolicy
	err = s.tx(ctx, p.OrgID, p.UserID, func(st Store, tx pgx.Tx) error {
		if err := in.checkRefs(ctx, st, p.OrgID); err != nil {
			return err
		}
		row, err := write(st, arg)
		if err != nil {
			return err
		}
		if out, err = toPolicy(row); err != nil {
			return err
		}
		return audit(ctx, tx, p.OrgID, p.UserID, action, "approval_policy", out.ID,
			map[string]any{"name": out.Name, "mode": out.Mode, "break_glass": out.BreakGlass})
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return ApprovalPolicy{}, fmt.Errorf("approval policy not found: %w", apperr.ErrNotFound)
	case isUniqueViolation(err):
		return ApprovalPolicy{}, fmt.Errorf("approval policy name is taken: %w", apperr.ErrAlreadyExists)
	}
	return out, err
}

// DeleteApprovalPolicy removes a policy; approvals made under it keep their history.
func (s *Service) DeleteApprovalPolicy(ctx context.Context, p principal.Principal, id uuid.UUID) error {
	return s.tx(ctx, p.OrgID, p.UserID, func(st Store, tx pgx.Tx) error {
		row, err := st.DeleteApprovalPolicy(ctx, dbq.DeleteApprovalPolicyParams{ID: id, OrgID: p.OrgID})
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("approval policy not found: %w", apperr.ErrNotFound)
		}
		if err != nil {
			return err
		}
		return audit(ctx, tx, p.OrgID, p.UserID, "approval_policy.delete", "approval_policy", id,
			map[string]any{"name": row.Name, "mode": row.Mode, "break_glass": row.BreakGlass})
	})
}
