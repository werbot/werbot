package policy

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// auditData returns the data of the audit events of action, as JSON texts.
func auditData(t *testing.T, e env, action string) []string {
	t.Helper()
	rows, err := e.pool.Query(context.Background(), "SELECT data::text FROM audit_event WHERE org_id = $1 AND action = $2 ORDER BY id", e.org, action)
	require.NoError(t, err)
	var out []string
	for rows.Next() {
		var s string
		require.NoError(t, rows.Scan(&s))
		out = append(out, s)
	}
	require.NoError(t, rows.Err())
	return out
}

func noRm(r uuid.UUID) CommandRuleInput {
	return CommandRuleInput{Name: "no-rm", Subjects: Subjects{AllUsers: true}, Selector: Selector{ResourceIDs: []uuid.UUID{r}},
		Classes: []string{"destructive"}, Effect: "deny", Message: "No rm"}
}

func TestCommandRuleCRUD(t *testing.T) {
	e := setup(t)
	r := e.resource(t, e.org)

	rule, err := e.svc.CreateCommandRule(e.ctx, e.org, noRm(r))
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, rule.ID)
	assert.Equal(t, noRm(r), rule.CommandRuleInput)
	assert.Equal(t, &e.owner, rule.CreatedBy)
	assert.False(t, rule.CreatedAt.IsZero())

	list, err := e.svc.ListCommandRules(e.ctx, e.org)
	require.NoError(t, err)
	assert.Equal(t, []CommandRule{rule}, list)

	_, err = e.svc.CreateCommandRule(e.ctx, e.org, noRm(r))
	assert.ErrorIs(t, err, apperr.ErrAlreadyExists)

	in := noRm(r)
	in.Effect = "approval"
	upd, err := e.svc.UpdateCommandRule(e.ctx, e.org, rule.ID, in)
	require.NoError(t, err)
	assert.Equal(t, "approval", upd.Effect)
	assert.Equal(t, rule.ID, upd.ID)
	_, err = e.svc.UpdateCommandRule(e.ctx, e.org, uuid.New(), in)
	assert.ErrorIs(t, err, apperr.ErrNotFound)

	require.NoError(t, e.svc.DeleteCommandRule(e.ctx, e.org, rule.ID))
	assert.ErrorIs(t, e.svc.DeleteCommandRule(e.ctx, e.org, rule.ID), apperr.ErrNotFound)
	list, err = e.svc.ListCommandRules(e.ctx, e.org)
	require.NoError(t, err)
	assert.Empty(t, list)

	assert.Equal(t, []string{`{"name": "no-rm", "effect": "deny", "classes": ["destructive"]}`}, auditData(t, e, "command_rule.create"))
	assert.Equal(t, []string{`{"name": "no-rm", "effect": "approval", "classes": ["destructive"]}`}, auditData(t, e, "command_rule.update"))
	assert.Equal(t, []string{`{"name": "no-rm", "effect": "approval", "classes": ["destructive"]}`}, auditData(t, e, "command_rule.delete"))
}

func TestCommandRuleRejects(t *testing.T) {
	e := setup(t)
	r := e.resource(t, e.org)
	orgB := dbtest.Org(t, e.pool)
	userB := dbtest.User(t, e.pool, orgB, "member")
	teamB := dbtest.Team(t, e.pool, orgB)
	resB := e.resource(t, orgB)
	for name, mut := range map[string]func(*CommandRuleInput){
		"empty name":      func(in *CommandRuleInput) { in.Name = "" },
		"long name":       func(in *CommandRuleInput) { in.Name = strings.Repeat("a", 64) },
		"empty subjects":  func(in *CommandRuleInput) { in.Subjects = Subjects{} },
		"both selectors":  func(in *CommandRuleInput) { in.Selector.Labels = map[string]string{"env": "prod"} },
		"no selector":     func(in *CommandRuleInput) { in.Selector = Selector{} },
		"too many ids":    func(in *CommandRuleInput) { in.Selector.ResourceIDs = make([]uuid.UUID, 101) },
		"no classes":      func(in *CommandRuleInput) { in.Classes = nil },
		"unknown class":   func(in *CommandRuleInput) { in.Classes = []string{"bogus"} },
		"duplicate class": func(in *CommandRuleInput) { in.Classes = []string{"read", "read"} },
		"unknown effect":  func(in *CommandRuleInput) { in.Effect = "maybe" },
		"control char":    func(in *CommandRuleInput) { in.Message = "No\nrm" },
		"long message":    func(in *CommandRuleInput) { in.Message = strings.Repeat("я", 257) },
		"other org user":  func(in *CommandRuleInput) { in.Subjects = Subjects{Users: []uuid.UUID{userB}} },
		"other org team":  func(in *CommandRuleInput) { in.Subjects = Subjects{Teams: []uuid.UUID{teamB}} },
		"other org res":   func(in *CommandRuleInput) { in.Selector.ResourceIDs = []uuid.UUID{resB} },
	} {
		in := noRm(r)
		mut(&in)
		_, err := e.svc.CreateCommandRule(e.ctx, e.org, in)
		assert.ErrorIs(t, err, apperr.ErrInvalidArgument, name)
	}

	in := noRm(r)
	in.Message = strings.Repeat("я", 256)
	_, err := e.svc.CreateCommandRule(e.ctx, e.org, in)
	assert.NoError(t, err, "256 printable runes")
}

func TestClassRuleCRUD(t *testing.T) {
	e := setup(t)
	rule, err := e.svc.CreateClassRule(e.ctx, e.org, "./deploy.sh", []string{"write"})
	require.NoError(t, err)
	assert.Equal(t, "./deploy.sh", rule.Pattern)
	assert.Equal(t, []string{"write"}, rule.Classes)
	assert.Equal(t, &e.owner, rule.CreatedBy)
	other, err := e.svc.CreateClassRule(e.ctx, e.org, "deploy*", []string{"destructive", "write"})
	require.NoError(t, err)

	list, err := e.svc.ListClassRules(e.ctx, e.org)
	require.NoError(t, err)
	assert.Equal(t, []ClassRule{rule, other}, list, "by pattern")

	for name, c := range map[string]struct {
		pattern string
		classes []string
	}{
		"bad glob":      {"[", []string{"write"}},
		"unknown class": {"x", []string{"bogus"}},
		"no classes":    {"x", nil},
		"dup class":     {"x", []string{"write", "write"}},
	} {
		_, err := e.svc.CreateClassRule(e.ctx, e.org, c.pattern, c.classes)
		assert.ErrorIs(t, err, apperr.ErrInvalidArgument, name)
	}
	_, err = e.svc.CreateClassRule(e.ctx, e.org, "./deploy.sh", []string{"read"})
	assert.ErrorIs(t, err, apperr.ErrAlreadyExists)

	require.NoError(t, e.svc.DeleteClassRule(e.ctx, e.org, rule.ID))
	assert.ErrorIs(t, e.svc.DeleteClassRule(e.ctx, e.org, rule.ID), apperr.ErrNotFound)
	assert.Equal(t, []string{`{"classes": ["write"], "pattern": "./deploy.sh"}`, `{"classes": ["destructive", "write"], "pattern": "deploy*"}`},
		auditData(t, e, "class_rule.create"))
	assert.Equal(t, []string{`{"classes": ["write"], "pattern": "./deploy.sh"}`}, auditData(t, e, "class_rule.delete"))
}
