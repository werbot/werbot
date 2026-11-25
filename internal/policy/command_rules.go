package policy

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/shellclass"
	"github.com/werbot/werbot/internal/policy/dbq"
)

// Limits of a command rule; label syntax is checked by the API (selector.proto).
const (
	maxNameRunes         = 63
	maxMessageRunes      = 256
	maxSelectorResources = 100
	maxSelectorLabels    = 8
	maxPatternBytes      = 255 // class rule pattern
)

// CommandRuleInput describes a command rule.
type CommandRuleInput struct {
	Name     string
	Subjects Subjects
	Selector Selector
	Classes  []string
	Effect   string // allow | deny | approval
	Message  string
}

// CommandRule is a stored command rule.
type CommandRule struct {
	ID uuid.UUID
	CommandRuleInput
	CreatedBy *uuid.UUID // nil: the creator was deleted
	CreatedAt time.Time
}

// ClassRule gives the commands matching Pattern the Classes.
type ClassRule struct {
	ID        uuid.UUID
	Pattern   string
	Classes   []string
	CreatedBy *uuid.UUID // nil: the creator was deleted
	CreatedAt time.Time
}

func validName(name string) error {
	if n := utf8.RuneCountInString(name); n < 1 || n > maxNameRunes {
		return invalid("name must have 1 to %d characters", maxNameRunes)
	}
	return nil
}

// validClasses accepts known classes without repeats, at least one.
func validClasses(classes []string) error {
	if len(classes) == 0 {
		return invalid("classes must not be empty")
	}
	for i, c := range classes {
		if !shellclass.Valid(shellclass.Class(c)) {
			return invalid("unknown class %q", c)
		}
		if slices.Contains(classes[:i], c) {
			return invalid("duplicate class %q", c)
		}
	}
	return nil
}

// validMessage accepts printable text (space included) of at most 256 runes.
func validMessage(m string) error {
	if !utf8.ValidString(m) || utf8.RuneCountInString(m) > maxMessageRunes {
		return invalid("message must be text of at most %d characters", maxMessageRunes)
	}
	if strings.IndexFunc(m, func(r rune) bool { return !unicode.IsPrint(r) }) >= 0 {
		return invalid("message must be printable text")
	}
	return nil
}

func (s Subjects) empty() bool {
	return len(s.Users) == 0 && len(s.Teams) == 0 && !s.AllUsers && !s.AllAgents
}

func (s Selector) validate() error {
	if (len(s.ResourceIDs) > 0) == (len(s.Labels) > 0) {
		return invalid("set exactly one of resources and labels in the selector")
	}
	if len(s.ResourceIDs) > maxSelectorResources || len(s.Labels) > maxSelectorLabels {
		return invalid("at most %d selector resources and %d labels", maxSelectorResources, maxSelectorLabels)
	}
	return nil
}

func (in CommandRuleInput) validate() error {
	if err := validTarget(in.Name, in.Subjects, in.Selector); err != nil {
		return err
	}
	if err := validClasses(in.Classes); err != nil {
		return err
	}
	if !slices.Contains(commandEffects, in.Effect) {
		return invalid("effect must be one of %s", strings.Join(commandEffects, ", "))
	}
	return validMessage(in.Message)
}

func toCommandRule(r dbq.CommandRule) (CommandRule, error) {
	out := CommandRule{ID: r.ID, CommandRuleInput: CommandRuleInput{Name: r.Name, Classes: r.Classes, Effect: r.Effect, Message: r.Message},
		CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt}
	if err := decodeTarget(r.Subjects, r.ResourceSelector, &out.Subjects, &out.Selector); err != nil {
		return CommandRule{}, fmt.Errorf("policy: decode command rule %s: %w", r.ID, err)
	}
	return out, nil
}

// snapshotCommandRules expands the selectors of rows over resources, by rule id.
func snapshotCommandRules(rows []dbq.CommandRule, resources []SnapshotResource) ([]SnapshotCommandRule, error) {
	out := make([]SnapshotCommandRule, 0, len(rows))
	for _, row := range rows {
		r, err := toCommandRule(row)
		if err != nil {
			return nil, err
		}
		out = append(out, SnapshotCommandRule{ID: r.ID, Subjects: r.Subjects, ResourceIDs: ExpandSelector(resources, r.Selector),
			Classes: r.Classes, Effect: r.Effect, Message: r.Message})
	}
	slices.SortFunc(out, func(x, y SnapshotCommandRule) int { return compareIDs(x.ID, y.ID) })
	return out, nil
}

func commandRuleChange(action string, r dbq.CommandRule) change {
	return change{Action: action, ID: r.ID, Data: map[string]any{"name": r.Name, "effect": r.Effect, "classes": r.Classes}}
}

// ListCommandRules returns the command rules of orgID by name.
func (s *Service) ListCommandRules(ctx context.Context, orgID uuid.UUID) ([]CommandRule, error) {
	var out []CommandRule
	err := dbx.InOrg(ctx, s.deps.Pool, orgID, actor(ctx), func(tx pgx.Tx) error {
		rows, err := newStore(tx).ListCommandRules(ctx, orgID)
		if err != nil {
			return err
		}
		out = make([]CommandRule, len(rows))
		for i, r := range rows {
			if out[i], err = toCommandRule(r); err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}

// CreateCommandRule adds a command rule to orgID.
func (s *Service) CreateCommandRule(ctx context.Context, orgID uuid.UUID, in CommandRuleInput) (CommandRule, error) {
	return s.writeCommandRule(ctx, orgID, in, "command_rule.create", func(st Store, arg dbq.UpdateCommandRuleParams, actor uuid.UUID) (dbq.CommandRule, error) {
		return st.CreateCommandRule(ctx, dbq.CreateCommandRuleParams{OrgID: arg.OrgID, Name: arg.Name, Subjects: arg.Subjects,
			ResourceSelector: arg.ResourceSelector, Classes: arg.Classes, Effect: arg.Effect, Message: arg.Message, CreatedBy: &actor})
	})
}

// UpdateCommandRule replaces the command rule id as a whole.
func (s *Service) UpdateCommandRule(ctx context.Context, orgID, id uuid.UUID, in CommandRuleInput) (CommandRule, error) {
	return s.writeCommandRule(ctx, orgID, in, "command_rule.update", func(st Store, arg dbq.UpdateCommandRuleParams, _ uuid.UUID) (dbq.CommandRule, error) {
		arg.ID = id
		return st.UpdateCommandRule(ctx, arg)
	})
}

// writeCommandRule validates in and runs write with its row in one audited transaction.
func (s *Service) writeCommandRule(ctx context.Context, orgID uuid.UUID, in CommandRuleInput, action string,
	write func(Store, dbq.UpdateCommandRuleParams, uuid.UUID) (dbq.CommandRule, error)) (CommandRule, error) {
	if err := in.validate(); err != nil {
		return CommandRule{}, err
	}
	arg := dbq.UpdateCommandRuleParams{OrgID: orgID, Name: in.Name, Classes: in.Classes, Effect: in.Effect, Message: in.Message}
	var err error
	if arg.Subjects, arg.ResourceSelector, err = encodeTarget(in.Subjects, in.Selector); err != nil {
		return CommandRule{}, err
	}
	var out CommandRule
	err = s.apply(ctx, orgID, "command rule", func(st Store, actor uuid.UUID) (change, error) {
		if err := checkRefs(ctx, st, orgID, in.Subjects, in.Selector); err != nil {
			return change{}, err
		}
		row, err := write(st, arg, actor)
		if err != nil {
			return change{}, err
		}
		out, err = toCommandRule(row)
		return commandRuleChange(action, row), err
	})
	return out, err
}

// DeleteCommandRule removes the command rule id.
func (s *Service) DeleteCommandRule(ctx context.Context, orgID, id uuid.UUID) error {
	return s.apply(ctx, orgID, "command rule", func(st Store, _ uuid.UUID) (change, error) {
		row, err := st.DeleteCommandRule(ctx, dbq.DeleteCommandRuleParams{OrgID: orgID, ID: id})
		return commandRuleChange("command_rule.delete", row), err
	})
}

func toClassRule(r dbq.CommandClassRule) ClassRule {
	return ClassRule{ID: r.ID, Pattern: r.Pattern, Classes: r.Classes, CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt}
}

func classRuleChange(action string, r dbq.CommandClassRule) change {
	return change{Action: action, ID: r.ID, Data: map[string]any{"pattern": r.Pattern, "classes": r.Classes}}
}

// ListClassRules returns the class rules of orgID by pattern.
func (s *Service) ListClassRules(ctx context.Context, orgID uuid.UUID) ([]ClassRule, error) {
	var out []ClassRule
	err := dbx.InOrg(ctx, s.deps.Pool, orgID, actor(ctx), func(tx pgx.Tx) error {
		rows, err := newStore(tx).ListClassRules(ctx, orgID)
		for _, r := range rows {
			out = append(out, toClassRule(r))
		}
		return err
	})
	return out, err
}

// CreateClassRule adds a class rule to orgID; the pattern must compile as
// the classifier compiles it.
func (s *Service) CreateClassRule(ctx context.Context, orgID uuid.UUID, pattern string, classes []string) (ClassRule, error) {
	if len(pattern) > maxPatternBytes {
		return ClassRule{}, invalid("pattern must have at most %d bytes", maxPatternBytes)
	}
	if err := validClasses(classes); err != nil {
		return ClassRule{}, err
	}
	if _, err := shellclass.New([]shellclass.Override{{Pattern: pattern, Classes: classesOf(classes)}}); err != nil {
		return ClassRule{}, fmt.Errorf("%w: %w", apperr.ErrInvalidArgument, err)
	}
	var out ClassRule
	err := s.apply(ctx, orgID, "class rule", func(st Store, actor uuid.UUID) (change, error) {
		row, err := st.CreateClassRule(ctx, dbq.CreateClassRuleParams{OrgID: orgID, Pattern: pattern, Classes: classes, CreatedBy: &actor})
		out = toClassRule(row)
		return classRuleChange("class_rule.create", row), err
	})
	return out, err
}

// DeleteClassRule removes the class rule id.
func (s *Service) DeleteClassRule(ctx context.Context, orgID, id uuid.UUID) error {
	return s.apply(ctx, orgID, "class rule", func(st Store, _ uuid.UUID) (change, error) {
		row, err := st.DeleteClassRule(ctx, dbq.DeleteClassRuleParams{OrgID: orgID, ID: id})
		return classRuleChange("class_rule.delete", row), err
	})
}
