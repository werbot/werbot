package policy

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/policy/dbq"
)

// TerminalRuleInput describes a terminal rule.
type TerminalRuleInput struct {
	Name     string
	Subjects Subjects
	Selector Selector
	Mode     string // console | deny
}

// TerminalRule is a stored terminal rule.
type TerminalRule struct {
	ID uuid.UUID
	TerminalRuleInput
	CreatedBy *uuid.UUID // nil: the creator was deleted
	CreatedAt time.Time
}

func (in TerminalRuleInput) validate() error {
	if err := validTarget(in.Name, in.Subjects, in.Selector); err != nil {
		return err
	}
	if !slices.Contains(terminalModes, in.Mode) {
		return invalid("mode must be one of %s", strings.Join(terminalModes, ", "))
	}
	return nil
}

func toTerminalRule(r dbq.TerminalRule) (TerminalRule, error) {
	out := TerminalRule{ID: r.ID, TerminalRuleInput: TerminalRuleInput{Name: r.Name, Mode: r.Mode}, CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt}
	if err := decodeTarget(r.Subjects, r.ResourceSelector, &out.Subjects, &out.Selector); err != nil {
		return TerminalRule{}, fmt.Errorf("policy: decode terminal rule %s: %w", r.ID, err)
	}
	return out, nil
}

func terminalRuleChange(action string, r dbq.TerminalRule) change {
	return change{Action: action, ID: r.ID, Data: map[string]any{"name": r.Name, "mode": r.Mode}}
}

// loadTerminal reads the terminal rules of snap.OrgID with their selectors
// expanded over snap.Resources, by id.
func loadTerminal(ctx context.Context, st Store, snap *Snapshot) error {
	rows, err := st.ListTerminalRules(ctx, snap.OrgID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		r, err := toTerminalRule(row)
		if err != nil {
			return err
		}
		snap.TerminalRules = append(snap.TerminalRules, SnapshotTerminalRule{ID: r.ID, Subjects: r.Subjects,
			ResourceIDs: ExpandSelector(snap.Resources, r.Selector), Mode: r.Mode})
	}
	return nil
}

// ListTerminalRules returns the terminal rules of orgID by name.
func (s *Service) ListTerminalRules(ctx context.Context, orgID uuid.UUID) ([]TerminalRule, error) {
	var out []TerminalRule
	err := dbx.InOrg(ctx, s.deps.Pool, orgID, actor(ctx), func(tx pgx.Tx) error {
		rows, err := newStore(tx).ListTerminalRules(ctx, orgID)
		if err != nil {
			return err
		}
		out = make([]TerminalRule, len(rows))
		for i, r := range rows {
			if out[i], err = toTerminalRule(r); err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}

// CreateTerminalRule adds a terminal rule to orgID.
func (s *Service) CreateTerminalRule(ctx context.Context, orgID uuid.UUID, in TerminalRuleInput) (TerminalRule, error) {
	if err := in.validate(); err != nil {
		return TerminalRule{}, err
	}
	subjects, selector, err := encodeTarget(in.Subjects, in.Selector)
	if err != nil {
		return TerminalRule{}, err
	}
	var out TerminalRule
	err = s.apply(ctx, orgID, "terminal rule", func(st Store, actor uuid.UUID) (change, error) {
		if err := checkRefs(ctx, st, orgID, in.Subjects, in.Selector); err != nil {
			return change{}, err
		}
		row, err := st.CreateTerminalRule(ctx, dbq.CreateTerminalRuleParams{OrgID: orgID, Name: in.Name, Subjects: subjects,
			ResourceSelector: selector, Mode: in.Mode, CreatedBy: &actor})
		if err != nil {
			return change{}, err
		}
		out, err = toTerminalRule(row)
		return terminalRuleChange("terminal_rule.create", row), err
	})
	return out, err
}

// DeleteTerminalRule removes the terminal rule id.
func (s *Service) DeleteTerminalRule(ctx context.Context, orgID, id uuid.UUID) error {
	return s.apply(ctx, orgID, "terminal rule", func(st Store, _ uuid.UUID) (change, error) {
		row, err := st.DeleteTerminalRule(ctx, dbq.DeleteTerminalRuleParams{OrgID: orgID, ID: id})
		return terminalRuleChange("terminal_rule.delete", row), err
	})
}
