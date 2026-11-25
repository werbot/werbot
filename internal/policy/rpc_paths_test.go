package policy

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

func TestPolicyRPCStage11b(t *testing.T) {
	e := setup(t)
	h := NewHandler(e.svc)
	member := dbtest.User(t, e.pool, e.org, "member")
	mctx := principal.With(context.Background(), principal.Principal{UserID: member, OrgID: e.org, Role: "member"})
	for name, call := range map[string]func(context.Context) error{
		"ListPathRules": func(c context.Context) error {
			_, err := h.ListPathRules(c, connect.NewRequest(&werbotv1.ListPathRulesRequest{}))
			return err
		},
		"CreatePathRule": func(c context.Context) error {
			_, err := h.CreatePathRule(c, connect.NewRequest(&werbotv1.CreatePathRuleRequest{}))
			return err
		},
		"UpdatePathRule": func(c context.Context) error {
			_, err := h.UpdatePathRule(c, connect.NewRequest(&werbotv1.UpdatePathRuleRequest{}))
			return err
		},
		"DeletePathRule": func(c context.Context) error {
			_, err := h.DeletePathRule(c, connect.NewRequest(&werbotv1.DeletePathRuleRequest{}))
			return err
		},
		"ListTerminalRules": func(c context.Context) error {
			_, err := h.ListTerminalRules(c, connect.NewRequest(&werbotv1.ListTerminalRulesRequest{}))
			return err
		},
		"CreateTerminalRule": func(c context.Context) error {
			_, err := h.CreateTerminalRule(c, connect.NewRequest(&werbotv1.CreateTerminalRuleRequest{}))
			return err
		},
		"DeleteTerminalRule": func(c context.Context) error {
			_, err := h.DeleteTerminalRule(c, connect.NewRequest(&werbotv1.DeleteTerminalRuleRequest{}))
			return err
		},
	} {
		assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(call(mctx)), name)
	}

	r := e.resource(t, e.org)
	_, err := h.CreatePathRule(e.ctx, connect.NewRequest(&werbotv1.CreatePathRuleRequest{Name: "x", Subjects: &werbotv1.Subjects{AllUsers: true},
		ResourceSelector: &werbotv1.ResourceSelector{ResourceIds: []string{r.String()}}, Paths: []string{"app/**"}, Access: "any", Effect: "only"}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	assert.Contains(t, err.Error(), "invalid path glob")

	_, err = h.TestCommand(e.ctx, connect.NewRequest(&werbotv1.TestCommandRequest{
		Subject: &werbotv1.TestCommandRequest_UserId{UserId: e.owner.String()}, ResourceId: r.String(), Command: "ls", Cwd: "x/../y"}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	assert.Contains(t, err.Error(), "invalid cwd")
}

func TestPolicyRPCStage11bRoundTrip(t *testing.T) {
	e := setup(t)
	h := NewHandler(e.svc)
	r := e.resource(t, e.org)
	e.bind(e.org, e.owner, r)
	subjects := &werbotv1.Subjects{AllUsers: true}
	sel := &werbotv1.ResourceSelector{ResourceIds: []string{r.String()}}

	created, err := h.CreatePathRule(e.ctx, connect.NewRequest(&werbotv1.CreatePathRuleRequest{Name: "app-only", Subjects: subjects,
		ResourceSelector: sel, Paths: []string{"~/app/**"}, Access: "any", Effect: "only"}))
	require.NoError(t, err)
	rule := created.Msg.GetRule()
	assert.Equal(t, e.owner.String(), rule.GetCreatedBy())
	assert.Equal(t, []string{r.String()}, rule.GetResourceSelector().GetResourceIds())
	assert.Equal(t, []string{"~/app/**"}, rule.GetPaths())
	updated, err := h.UpdatePathRule(e.ctx, connect.NewRequest(&werbotv1.UpdatePathRuleRequest{Id: rule.GetId(), Name: "app-only",
		Subjects: subjects, ResourceSelector: sel, Paths: []string{"~/app/**", "/srv/**"}, Access: "any", Effect: "only", Message: "Stay in app"}))
	require.NoError(t, err)
	assert.Equal(t, "Stay in app", updated.Msg.GetRule().GetMessage())
	rules, err := h.ListPathRules(e.ctx, connect.NewRequest(&werbotv1.ListPathRulesRequest{}))
	require.NoError(t, err)
	assert.Equal(t, []string{"~/app/**", "/srv/**"}, rules.Msg.GetRules()[0].GetPaths())

	res, err := h.TestCommand(withClientIP(e.ctx, "10.1.2.3"), connect.NewRequest(&werbotv1.TestCommandRequest{
		Subject: &werbotv1.TestCommandRequest_UserId{UserId: e.owner.String()}, ResourceId: r.String(), Command: "cat x /etc/hosts", Cwd: "/srv"}))
	require.NoError(t, err)
	assert.False(t, res.Msg.GetAllow())
	assert.Equal(t, []string{"/etc/hosts", "/srv/x"}, res.Msg.GetPaths())
	assert.Equal(t, []string{rule.GetId()}, res.Msg.GetPathOutside())
	assert.Empty(t, res.Msg.GetPathHits())
	assert.Equal(t, "Stay in app", res.Msg.GetMessage())
	_, err = h.DeletePathRule(e.ctx, connect.NewRequest(&werbotv1.DeletePathRuleRequest{Id: rule.GetId()}))
	require.NoError(t, err)

	term, err := h.CreateTerminalRule(e.ctx, connect.NewRequest(&werbotv1.CreateTerminalRuleRequest{Name: "console", Subjects: subjects,
		ResourceSelector: sel, Mode: "console"}))
	require.NoError(t, err)
	assert.Equal(t, "console", term.Msg.GetRule().GetMode())
	assert.Equal(t, e.owner.String(), term.Msg.GetRule().GetCreatedBy())
	terms, err := h.ListTerminalRules(e.ctx, connect.NewRequest(&werbotv1.ListTerminalRulesRequest{}))
	require.NoError(t, err)
	assert.Equal(t, term.Msg.GetRule().GetId(), terms.Msg.GetRules()[0].GetId())
	_, err = h.DeleteTerminalRule(e.ctx, connect.NewRequest(&werbotv1.DeleteTerminalRuleRequest{Id: term.Msg.GetRule().GetId()}))
	require.NoError(t, err)

	_, err = h.DeletePathRule(e.ctx, connect.NewRequest(&werbotv1.DeletePathRuleRequest{Id: "x"}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	_, err = h.DeleteTerminalRule(e.ctx, connect.NewRequest(&werbotv1.DeleteTerminalRuleRequest{Id: rule.GetId()}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
