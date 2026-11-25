package policy

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/policy/dbq"
)

// MFARuleInput describes an MFA rule: the people among Subjects confirm a
// second factor younger than MaxAge (whole seconds, 30s to 24h) of
// Strength before they connect to the resources of Selector.
type MFARuleInput struct {
	Name     string
	Subjects Subjects
	Selector Selector
	MaxAge   time.Duration
	Strength string // any | webauthn
}

// MFARule is a stored MFA rule.
type MFARule struct {
	ID uuid.UUID
	MFARuleInput
	CreatedAt time.Time
}

func (in MFARuleInput) validate() error {
	if err := validTarget(in.Name, in.Subjects, in.Selector); err != nil {
		return err
	}
	if in.Subjects.AllAgents {
		return invalid("mfa rules apply to users only")
	}
	if in.MaxAge%time.Second != 0 {
		return invalid("max_age must be whole seconds")
	}
	return validMFA(int64(in.MaxAge/time.Second), in.Strength)
}

// maxAgeOf is a stored max_age; the check constraint keeps it within a day.
func maxAgeOf(i pgtype.Interval) time.Duration {
	return time.Duration(i.Days)*24*time.Hour + time.Duration(i.Microseconds)*time.Microsecond
}

func toMFARule(r dbq.MfaRule) (MFARule, error) {
	out := MFARule{ID: r.ID, MFARuleInput: MFARuleInput{Name: r.Name, MaxAge: maxAgeOf(r.MaxAge), Strength: r.Strength}, CreatedAt: r.CreatedAt}
	if err := decodeTarget(r.Subjects, r.ResourceSelector, &out.Subjects, &out.Selector); err != nil {
		return MFARule{}, fmt.Errorf("policy: decode mfa rule %s: %w", r.ID, err)
	}
	return out, nil
}

func mfaRuleChange(action string, r dbq.MfaRule) change {
	return change{Action: action, ID: r.ID, Data: map[string]any{"name": r.Name, "strength": r.Strength,
		"max_age_seconds": int64(maxAgeOf(r.MaxAge) / time.Second)}}
}

// loadMFA reads the MFA rules of snap.OrgID with their selectors expanded
// over snap.Resources, by name.
func loadMFA(ctx context.Context, st Store, snap *Snapshot) error {
	rows, err := st.ListMFARules(ctx, snap.OrgID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		r, err := toMFARule(row)
		if err != nil {
			return err
		}
		snap.MFARules = append(snap.MFARules, SnapshotMFARule{ID: r.ID, Subjects: r.Subjects, ResourceIDs: ExpandSelector(snap.Resources, r.Selector),
			MaxAgeSeconds: int64(r.MaxAge / time.Second), Strength: r.Strength})
	}
	return nil
}

// ListMFARules returns the MFA rules of orgID by name.
func (s *Service) ListMFARules(ctx context.Context, orgID uuid.UUID) ([]MFARule, error) {
	var out []MFARule
	err := dbx.InOrg(ctx, s.deps.Pool, orgID, actor(ctx), func(tx pgx.Tx) error {
		rows, err := newStore(tx).ListMFARules(ctx, orgID)
		if err != nil {
			return err
		}
		out = make([]MFARule, len(rows))
		for i, r := range rows {
			if out[i], err = toMFARule(r); err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}

// CreateMFARule adds an MFA rule to orgID.
func (s *Service) CreateMFARule(ctx context.Context, orgID uuid.UUID, in MFARuleInput) (MFARule, error) {
	return s.writeMFARule(ctx, orgID, in, "mfa_rule.create", func(st Store, arg dbq.UpdateMFARuleParams, actor uuid.UUID) (dbq.MfaRule, error) {
		return st.CreateMFARule(ctx, dbq.CreateMFARuleParams{OrgID: arg.OrgID, Name: arg.Name, Subjects: arg.Subjects,
			ResourceSelector: arg.ResourceSelector, MaxAge: arg.MaxAge, Strength: arg.Strength, CreatedBy: &actor})
	})
}

// UpdateMFARule replaces the MFA rule id as a whole.
func (s *Service) UpdateMFARule(ctx context.Context, orgID, id uuid.UUID, in MFARuleInput) (MFARule, error) {
	return s.writeMFARule(ctx, orgID, in, "mfa_rule.update", func(st Store, arg dbq.UpdateMFARuleParams, _ uuid.UUID) (dbq.MfaRule, error) {
		arg.ID = id
		return st.UpdateMFARule(ctx, arg)
	})
}

// writeMFARule validates in and runs write with its row in one audited transaction.
func (s *Service) writeMFARule(ctx context.Context, orgID uuid.UUID, in MFARuleInput, action string,
	write func(Store, dbq.UpdateMFARuleParams, uuid.UUID) (dbq.MfaRule, error)) (MFARule, error) {
	if err := in.validate(); err != nil {
		return MFARule{}, err
	}
	arg := dbq.UpdateMFARuleParams{OrgID: orgID, Name: in.Name, Strength: in.Strength,
		MaxAge: pgtype.Interval{Microseconds: in.MaxAge.Microseconds(), Valid: true}}
	var err error
	if arg.Subjects, arg.ResourceSelector, err = encodeTarget(in.Subjects, in.Selector); err != nil {
		return MFARule{}, err
	}
	var out MFARule
	err = s.apply(ctx, orgID, "mfa rule", func(st Store, actor uuid.UUID) (change, error) {
		if err := checkRefs(ctx, st, orgID, in.Subjects, in.Selector); err != nil {
			return change{}, err
		}
		row, err := write(st, arg, actor)
		if err != nil {
			return change{}, err
		}
		out, err = toMFARule(row)
		return mfaRuleChange(action, row), err
	})
	return out, err
}

// DeleteMFARule removes the MFA rule id.
func (s *Service) DeleteMFARule(ctx context.Context, orgID, id uuid.UUID) error {
	return s.apply(ctx, orgID, "mfa rule", func(st Store, _ uuid.UUID) (change, error) {
		row, err := st.DeleteMFARule(ctx, dbq.DeleteMFARuleParams{OrgID: orgID, ID: id})
		return mfaRuleChange("mfa_rule.delete", row), err
	})
}
