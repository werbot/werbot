package policy

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

func appOnly(r uuid.UUID) PathRuleInput {
	return PathRuleInput{Name: "app-only", Subjects: Subjects{AllUsers: true}, Selector: Selector{ResourceIDs: []uuid.UUID{r}},
		Paths: []string{"~/app/**"}, Access: "any", Effect: "only"}
}

func TestPathRuleCRUD(t *testing.T) {
	e := setup(t)
	r := e.resource(t, e.org)

	rule, err := e.svc.CreatePathRule(e.ctx, e.org, appOnly(r))
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, rule.ID)
	assert.Equal(t, appOnly(r), rule.PathRuleInput)
	assert.Equal(t, &e.owner, rule.CreatedBy)
	assert.False(t, rule.CreatedAt.IsZero())

	list, err := e.svc.ListPathRules(e.ctx, e.org)
	require.NoError(t, err)
	assert.Equal(t, []PathRule{rule}, list)

	_, err = e.svc.CreatePathRule(e.ctx, e.org, appOnly(r))
	assert.ErrorIs(t, err, apperr.ErrAlreadyExists)

	in := appOnly(r)
	in.Effect, in.Access, in.Paths, in.Message = "deny", "write", []string{"/etc/**", "~"}, "Read only"
	upd, err := e.svc.UpdatePathRule(e.ctx, e.org, rule.ID, in)
	require.NoError(t, err)
	assert.Equal(t, in, upd.PathRuleInput)
	assert.Equal(t, rule.ID, upd.ID)
	_, err = e.svc.UpdatePathRule(e.ctx, e.org, uuid.New(), in)
	assert.ErrorIs(t, err, apperr.ErrNotFound)

	require.NoError(t, e.svc.DeletePathRule(e.ctx, e.org, rule.ID))
	assert.ErrorIs(t, e.svc.DeletePathRule(e.ctx, e.org, rule.ID), apperr.ErrNotFound)
	list, err = e.svc.ListPathRules(e.ctx, e.org)
	require.NoError(t, err)
	assert.Empty(t, list)

	assert.Equal(t, []string{`{"name": "app-only", "paths": ["~/app/**"], "access": "any", "effect": "only"}`}, auditData(t, e, "path_rule.create"))
	updated := `{"name": "app-only", "paths": ["/etc/**", "~"], "access": "write", "effect": "deny"}`
	assert.Equal(t, []string{updated}, auditData(t, e, "path_rule.update"))
	assert.Equal(t, []string{updated}, auditData(t, e, "path_rule.delete"))
}

func TestPathRuleRejects(t *testing.T) {
	e := setup(t)
	r := e.resource(t, e.org)
	orgB := dbtest.Org(t, e.pool)
	userB := dbtest.User(t, e.pool, orgB, "member")
	teamB := dbtest.Team(t, e.pool, orgB)
	resB := e.resource(t, orgB)
	many := make([]string, MaxRulePaths+1)
	for i := range many {
		many[i] = "/srv/" + strings.Repeat("a", i+1)
	}
	for name, mut := range map[string]func(*PathRuleInput){
		"empty name":      func(in *PathRuleInput) { in.Name = "" },
		"empty subjects":  func(in *PathRuleInput) { in.Subjects = Subjects{} },
		"no selector":     func(in *PathRuleInput) { in.Selector = Selector{} },
		"no paths":        func(in *PathRuleInput) { in.Paths = nil },
		"too many paths":  func(in *PathRuleInput) { in.Paths = many },
		"relative":        func(in *PathRuleInput) { in.Paths = []string{"app/**"} },
		"unclean":         func(in *PathRuleInput) { in.Paths = []string{"/a/../b"} },
		"partial **":      func(in *PathRuleInput) { in.Paths = []string{"/a/**b"} },
		"user home":       func(in *PathRuleInput) { in.Paths = []string{"~user/x"} },
		"duplicate path":  func(in *PathRuleInput) { in.Paths = []string{"/srv/**", "/srv/**"} },
		"unknown access":  func(in *PathRuleInput) { in.Access = "exec" },
		"unknown effect":  func(in *PathRuleInput) { in.Effect = "allow" },
		"control char":    func(in *PathRuleInput) { in.Message = "No\nway" },
		"other org user":  func(in *PathRuleInput) { in.Subjects = Subjects{Users: []uuid.UUID{userB}} },
		"other org team":  func(in *PathRuleInput) { in.Subjects = Subjects{Teams: []uuid.UUID{teamB}} },
		"other org res":   func(in *PathRuleInput) { in.Selector.ResourceIDs = []uuid.UUID{resB} },
		"both selectors":  func(in *PathRuleInput) { in.Selector.Labels = map[string]string{"env": "prod"} },
		"long path glob":  func(in *PathRuleInput) { in.Paths = []string{"/" + strings.Repeat("a", 1024)} },
		"empty path glob": func(in *PathRuleInput) { in.Paths = []string{""} },
	} {
		in := appOnly(r)
		mut(&in)
		_, err := e.svc.CreatePathRule(e.ctx, e.org, in)
		assert.ErrorIs(t, err, apperr.ErrInvalidArgument, name)
	}

	in := appOnly(r)
	in.Paths = []string{"/" + strings.Repeat("a", 1023)}
	_, err := e.svc.CreatePathRule(e.ctx, e.org, in)
	assert.NoError(t, err, "1024-byte glob")
	_, err = e.svc.CreatePathRule(e.ctx, e.org, PathRuleInput{Name: "x", Subjects: Subjects{AllUsers: true},
		Selector: Selector{ResourceIDs: []uuid.UUID{r}}, Paths: []string{"app/**"}, Access: "any", Effect: "only"})
	require.ErrorIs(t, err, apperr.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "invalid path glob")
}

func TestSnapshotIncludesStage11b(t *testing.T) {
	e := setup(t)
	prod := e.resource(t, e.org)
	e.resource(t, e.org)
	e.res[e.org][0].Labels = map[string]string{"env": "prod"}
	in := appOnly(prod)
	in.Selector = Selector{Labels: map[string]string{"env": "prod"}}
	path, err := e.svc.CreatePathRule(e.ctx, e.org, in)
	require.NoError(t, err)
	term, err := e.svc.CreateTerminalRule(e.ctx, e.org, consoleOnly(prod))
	require.NoError(t, err)

	snap, err := e.svc.Snapshot(e.ctx, e.org)
	require.NoError(t, err)
	assert.Equal(t, []SnapshotPathRule{{ID: path.ID, Subjects: in.Subjects, ResourceIDs: []uuid.UUID{prod}, Paths: in.Paths,
		Access: "any", Effect: "only"}}, snap.PathRules)
	assert.Equal(t, []SnapshotTerminalRule{{ID: term.ID, Subjects: term.Subjects, ResourceIDs: []uuid.UUID{prod}, Mode: "console"}},
		snap.TerminalRules)
	_, text, err := Compile(snap)
	require.NoError(t, err)
	assert.Contains(t, text, `@id("path:`+path.ID.String()+`")`)
	assert.Contains(t, text, `@id("terminal:`+term.ID.String()+`")`)
}
