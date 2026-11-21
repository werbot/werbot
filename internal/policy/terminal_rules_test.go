package policy

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

func consoleOnly(r uuid.UUID) TerminalRuleInput {
	return TerminalRuleInput{Name: "console", Subjects: Subjects{AllUsers: true}, Selector: Selector{ResourceIDs: []uuid.UUID{r}}, Mode: "console"}
}

func TestTerminalRuleCRUD(t *testing.T) {
	e := setup(t)
	r := e.resource(t, e.org)

	rule, err := e.svc.CreateTerminalRule(e.ctx, e.org, consoleOnly(r))
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, rule.ID)
	assert.Equal(t, consoleOnly(r), rule.TerminalRuleInput)
	assert.Equal(t, &e.owner, rule.CreatedBy)
	assert.False(t, rule.CreatedAt.IsZero())

	deny := consoleOnly(r)
	deny.Name, deny.Mode = "no-shell", "deny"
	other, err := e.svc.CreateTerminalRule(e.ctx, e.org, deny)
	require.NoError(t, err)
	list, err := e.svc.ListTerminalRules(e.ctx, e.org)
	require.NoError(t, err)
	assert.Equal(t, []TerminalRule{rule, other}, list, "by name")

	_, err = e.svc.CreateTerminalRule(e.ctx, e.org, consoleOnly(r))
	assert.ErrorIs(t, err, apperr.ErrAlreadyExists)

	require.NoError(t, e.svc.DeleteTerminalRule(e.ctx, e.org, rule.ID))
	assert.ErrorIs(t, e.svc.DeleteTerminalRule(e.ctx, e.org, rule.ID), apperr.ErrNotFound)
	list, err = e.svc.ListTerminalRules(e.ctx, e.org)
	require.NoError(t, err)
	assert.Equal(t, []TerminalRule{other}, list)

	assert.Equal(t, []string{`{"mode": "console", "name": "console"}`, `{"mode": "deny", "name": "no-shell"}`},
		auditData(t, e, "terminal_rule.create"))
	assert.Equal(t, []string{`{"mode": "console", "name": "console"}`}, auditData(t, e, "terminal_rule.delete"))
}

func TestTerminalRuleRejects(t *testing.T) {
	e := setup(t)
	r := e.resource(t, e.org)
	orgB := dbtest.Org(t, e.pool)
	userB := dbtest.User(t, e.pool, orgB, "member")
	resB := e.resource(t, orgB)
	for name, mut := range map[string]func(*TerminalRuleInput){
		"empty name":     func(in *TerminalRuleInput) { in.Name = "" },
		"empty subjects": func(in *TerminalRuleInput) { in.Subjects = Subjects{} },
		"no selector":    func(in *TerminalRuleInput) { in.Selector = Selector{} },
		"unknown mode":   func(in *TerminalRuleInput) { in.Mode = "shell" },
		"other org user": func(in *TerminalRuleInput) { in.Subjects = Subjects{Users: []uuid.UUID{userB}} },
		"other org res":  func(in *TerminalRuleInput) { in.Selector.ResourceIDs = []uuid.UUID{resB} },
	} {
		in := consoleOnly(r)
		mut(&in)
		_, err := e.svc.CreateTerminalRule(e.ctx, e.org, in)
		assert.ErrorIs(t, err, apperr.ErrInvalidArgument, name)
	}
}
