package policy

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/shellclass"
	"github.com/werbot/werbot/internal/policy/dbq"
)

// MaxRulePaths bounds the globs of one path rule.
const MaxRulePaths = 32

// PathRuleInput describes a path rule.
type PathRuleInput struct {
	Name     string
	Subjects Subjects
	Selector Selector
	Paths    []string // globs (shellclass.ValidPathGlob)
	Access   string   // read | write | any
	Effect   string   // only | deny | approval
	Message  string
}

// PathRule is a stored path rule.
type PathRule struct {
	ID uuid.UUID
	PathRuleInput
	CreatedBy *uuid.UUID // nil: the creator was deleted
	CreatedAt time.Time
}

// validPaths accepts 1 to MaxRulePaths valid globs without repeats; the glob
// error text reaches the client.
func validPaths(paths []string) error {
	if len(paths) == 0 || len(paths) > MaxRulePaths {
		return invalid("paths must have 1 to %d globs", MaxRulePaths)
	}
	for i, p := range paths {
		if err := shellclass.ValidPathGlob(p); err != nil {
			return fmt.Errorf("%w: %w", apperr.ErrInvalidArgument, err)
		}
		if slices.Contains(paths[:i], p) {
			return invalid("duplicate path %q", p)
		}
	}
	return nil
}

func (in PathRuleInput) validate() error {
	if err := validTarget(in.Name, in.Subjects, in.Selector); err != nil {
		return err
	}
	if err := validPaths(in.Paths); err != nil {
		return err
	}
	if !slices.Contains(pathAccesses, in.Access) {
		return invalid("access must be one of %s", strings.Join(pathAccesses, ", "))
	}
	if !slices.Contains(pathEffects, in.Effect) {
		return invalid("effect must be one of %s", strings.Join(pathEffects, ", "))
	}
	return validMessage(in.Message)
}

func toPathRule(r dbq.PathRule) (PathRule, error) {
	out := PathRule{ID: r.ID, PathRuleInput: PathRuleInput{Name: r.Name, Paths: r.Paths, Access: r.Access, Effect: r.Effect, Message: r.Message},
		CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt}
	if err := decodeTarget(r.Subjects, r.ResourceSelector, &out.Subjects, &out.Selector); err != nil {
		return PathRule{}, fmt.Errorf("policy: decode path rule %s: %w", r.ID, err)
	}
	return out, nil
}

func pathRuleChange(action string, r dbq.PathRule) change {
	return change{Action: action, ID: r.ID, Data: map[string]any{"name": r.Name, "effect": r.Effect, "access": r.Access, "paths": r.Paths}}
}

// loadPaths reads the path and terminal rules of snap.OrgID with their
// selectors expanded over snap.Resources, by id.
func loadPaths(ctx context.Context, st Store, snap *Snapshot) error {
	paths, err := st.ListPathRules(ctx, snap.OrgID)
	if err != nil {
		return err
	}
	for _, row := range paths {
		r, err := toPathRule(row)
		if err != nil {
			return err
		}
		snap.PathRules = append(snap.PathRules, SnapshotPathRule{ID: r.ID, Subjects: r.Subjects, ResourceIDs: ExpandSelector(snap.Resources, r.Selector),
			Paths: r.Paths, Access: r.Access, Effect: r.Effect, Message: r.Message})
	}
	slices.SortFunc(snap.PathRules, func(x, y SnapshotPathRule) int { return compareIDs(x.ID, y.ID) })
	return loadTerminal(ctx, st, snap)
}

// ListPathRules returns the path rules of orgID by name.
func (s *Service) ListPathRules(ctx context.Context, orgID uuid.UUID) ([]PathRule, error) {
	var out []PathRule
	err := dbx.InOrg(ctx, s.deps.Pool, orgID, actor(ctx), func(tx pgx.Tx) error {
		rows, err := newStore(tx).ListPathRules(ctx, orgID)
		if err != nil {
			return err
		}
		out = make([]PathRule, len(rows))
		for i, r := range rows {
			if out[i], err = toPathRule(r); err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}

// CreatePathRule adds a path rule to orgID.
func (s *Service) CreatePathRule(ctx context.Context, orgID uuid.UUID, in PathRuleInput) (PathRule, error) {
	return s.writePathRule(ctx, orgID, in, "path_rule.create", func(st Store, arg dbq.UpdatePathRuleParams, actor uuid.UUID) (dbq.PathRule, error) {
		return st.CreatePathRule(ctx, dbq.CreatePathRuleParams{OrgID: arg.OrgID, Name: arg.Name, Subjects: arg.Subjects,
			ResourceSelector: arg.ResourceSelector, Paths: arg.Paths, Access: arg.Access, Effect: arg.Effect, Message: arg.Message, CreatedBy: &actor})
	})
}

// UpdatePathRule replaces the path rule id as a whole.
func (s *Service) UpdatePathRule(ctx context.Context, orgID, id uuid.UUID, in PathRuleInput) (PathRule, error) {
	return s.writePathRule(ctx, orgID, in, "path_rule.update", func(st Store, arg dbq.UpdatePathRuleParams, _ uuid.UUID) (dbq.PathRule, error) {
		arg.ID = id
		return st.UpdatePathRule(ctx, arg)
	})
}

// writePathRule validates in and runs write with its row in one audited transaction.
func (s *Service) writePathRule(ctx context.Context, orgID uuid.UUID, in PathRuleInput, action string,
	write func(Store, dbq.UpdatePathRuleParams, uuid.UUID) (dbq.PathRule, error)) (PathRule, error) {
	if err := in.validate(); err != nil {
		return PathRule{}, err
	}
	arg := dbq.UpdatePathRuleParams{OrgID: orgID, Name: in.Name, Paths: in.Paths, Access: in.Access, Effect: in.Effect, Message: in.Message}
	var err error
	if arg.Subjects, arg.ResourceSelector, err = encodeTarget(in.Subjects, in.Selector); err != nil {
		return PathRule{}, err
	}
	var out PathRule
	err = s.apply(ctx, orgID, "path rule", func(st Store, actor uuid.UUID) (change, error) {
		if err := checkRefs(ctx, st, orgID, in.Subjects, in.Selector); err != nil {
			return change{}, err
		}
		row, err := write(st, arg, actor)
		if err != nil {
			return change{}, err
		}
		out, err = toPathRule(row)
		return pathRuleChange(action, row), err
	})
	return out, err
}

// DeletePathRule removes the path rule id.
func (s *Service) DeletePathRule(ctx context.Context, orgID, id uuid.UUID) error {
	return s.apply(ctx, orgID, "path rule", func(st Store, _ uuid.UUID) (change, error) {
		row, err := st.DeletePathRule(ctx, dbq.DeletePathRuleParams{OrgID: orgID, ID: id})
		return pathRuleChange("path_rule.delete", row), err
	})
}
