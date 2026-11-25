package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

// withClientIP returns ctx with the client address the server middleware stores.
func withClientIP(ctx context.Context, ip string) context.Context {
	r := httptest.NewRequestWithContext(ctx, http.MethodPost, "/", nil)
	r.RemoteAddr = ip + ":1234"
	clientip.Middleware(nil)(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) { ctx = req.Context() })).ServeHTTP(nil, r)
	return ctx
}

func TestPolicyRPCStage11(t *testing.T) {
	e := setup(t)
	h := NewHandler(e.svc)
	member := dbtest.User(t, e.pool, e.org, "member")
	mctx := principal.With(context.Background(), principal.Principal{UserID: member, OrgID: e.org, Role: "member"})
	for name, call := range map[string]func(context.Context) error{
		"ListCommandRules": func(c context.Context) error {
			_, err := h.ListCommandRules(c, connect.NewRequest(&werbotv1.ListCommandRulesRequest{}))
			return err
		},
		"CreateCommandRule": func(c context.Context) error {
			_, err := h.CreateCommandRule(c, connect.NewRequest(&werbotv1.CreateCommandRuleRequest{}))
			return err
		},
		"UpdateCommandRule": func(c context.Context) error {
			_, err := h.UpdateCommandRule(c, connect.NewRequest(&werbotv1.UpdateCommandRuleRequest{}))
			return err
		},
		"DeleteCommandRule": func(c context.Context) error {
			_, err := h.DeleteCommandRule(c, connect.NewRequest(&werbotv1.DeleteCommandRuleRequest{}))
			return err
		},
		"ListClassRules": func(c context.Context) error {
			_, err := h.ListClassRules(c, connect.NewRequest(&werbotv1.ListClassRulesRequest{}))
			return err
		},
		"CreateClassRule": func(c context.Context) error {
			_, err := h.CreateClassRule(c, connect.NewRequest(&werbotv1.CreateClassRuleRequest{}))
			return err
		},
		"DeleteClassRule": func(c context.Context) error {
			_, err := h.DeleteClassRule(c, connect.NewRequest(&werbotv1.DeleteClassRuleRequest{}))
			return err
		},
		"ListCustomPolicies": func(c context.Context) error {
			_, err := h.ListCustomPolicies(c, connect.NewRequest(&werbotv1.ListCustomPoliciesRequest{}))
			return err
		},
		"CreateCustomPolicy": func(c context.Context) error {
			_, err := h.CreateCustomPolicy(c, connect.NewRequest(&werbotv1.CreateCustomPolicyRequest{}))
			return err
		},
		"UpdateCustomPolicy": func(c context.Context) error {
			_, err := h.UpdateCustomPolicy(c, connect.NewRequest(&werbotv1.UpdateCustomPolicyRequest{}))
			return err
		},
		"DeleteCustomPolicy": func(c context.Context) error {
			_, err := h.DeleteCustomPolicy(c, connect.NewRequest(&werbotv1.DeleteCustomPolicyRequest{}))
			return err
		},
		"TestCommand": func(c context.Context) error {
			_, err := h.TestCommand(c, connect.NewRequest(&werbotv1.TestCommandRequest{}))
			return err
		},
	} {
		assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(call(mctx)), name)
	}

	_, err := h.CreateCustomPolicy(e.ctx, connect.NewRequest(&werbotv1.CreateCustomPolicyRequest{Name: "x", Cedar: "permit ("}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	assert.Contains(t, err.Error(), "1:")

	_, err = h.TestCommand(e.ctx, connect.NewRequest(&werbotv1.TestCommandRequest{
		Subject: &werbotv1.TestCommandRequest_AgentId{AgentId: member.String()}, ResourceId: member.String(), Command: "ls"}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "no such agent")
}

func TestPolicyRPCStage11RoundTrip(t *testing.T) {
	e := setup(t)
	h := NewHandler(e.svc)
	r := e.resource(t, e.org)
	e.bind(e.org, e.owner, r)
	subjects := &werbotv1.Subjects{AllUsers: true}
	sel := &werbotv1.ResourceSelector{ResourceIds: []string{r.String()}}

	created, err := h.CreateCommandRule(e.ctx, connect.NewRequest(&werbotv1.CreateCommandRuleRequest{Name: "no-rm", Subjects: subjects,
		ResourceSelector: sel, Classes: []string{"destructive"}, Effect: "deny", Message: "No rm"}))
	require.NoError(t, err)
	rule := created.Msg.GetRule()
	assert.Equal(t, e.owner.String(), rule.GetCreatedBy())
	assert.Equal(t, []string{r.String()}, rule.GetResourceSelector().GetResourceIds())
	assert.True(t, rule.GetSubjects().GetAllUsers())
	updated, err := h.UpdateCommandRule(e.ctx, connect.NewRequest(&werbotv1.UpdateCommandRuleRequest{Id: rule.GetId(), Name: "no-rm",
		Subjects: subjects, ResourceSelector: sel, Classes: []string{"destructive"}, Effect: "deny", Message: "No rm here"}))
	require.NoError(t, err)
	assert.Equal(t, "No rm here", updated.Msg.GetRule().GetMessage())
	rules, err := h.ListCommandRules(e.ctx, connect.NewRequest(&werbotv1.ListCommandRulesRequest{}))
	require.NoError(t, err)
	assert.Len(t, rules.Msg.GetRules(), 1)

	res, err := h.TestCommand(withClientIP(e.ctx, "10.1.2.3"), connect.NewRequest(&werbotv1.TestCommandRequest{
		Subject: &werbotv1.TestCommandRequest_UserId{UserId: e.owner.String()}, ResourceId: r.String(), Command: "rm -rf /"}))
	require.NoError(t, err)
	assert.Equal(t, []string{"destructive"}, res.Msg.GetClasses())
	assert.False(t, res.Msg.GetAllow())
	assert.Equal(t, []string{"command:" + rule.GetId()}, res.Msg.GetReasons())
	assert.Equal(t, "No rm here", res.Msg.GetMessage())
	_, err = h.DeleteCommandRule(e.ctx, connect.NewRequest(&werbotv1.DeleteCommandRuleRequest{Id: rule.GetId()}))
	require.NoError(t, err)

	class, err := h.CreateClassRule(e.ctx, connect.NewRequest(&werbotv1.CreateClassRuleRequest{Pattern: "./deploy.sh", Classes: []string{"write"}}))
	require.NoError(t, err)
	classes, err := h.ListClassRules(e.ctx, connect.NewRequest(&werbotv1.ListClassRulesRequest{}))
	require.NoError(t, err)
	assert.Equal(t, "./deploy.sh", classes.Msg.GetRules()[0].GetPattern())
	_, err = h.DeleteClassRule(e.ctx, connect.NewRequest(&werbotv1.DeleteClassRuleRequest{Id: class.Msg.GetRule().GetId()}))
	require.NoError(t, err)

	pol, err := h.CreateCustomPolicy(e.ctx, connect.NewRequest(&werbotv1.CreateCustomPolicyRequest{Name: "review", Cedar: approvalCedar, Enabled: true}))
	require.NoError(t, err)
	_, err = h.UpdateCustomPolicy(e.ctx, connect.NewRequest(&werbotv1.UpdateCustomPolicyRequest{Id: pol.Msg.GetPolicy().GetId(), Name: "review",
		Cedar: approvalCedar}))
	require.NoError(t, err)
	pols, err := h.ListCustomPolicies(e.ctx, connect.NewRequest(&werbotv1.ListCustomPoliciesRequest{}))
	require.NoError(t, err)
	assert.False(t, pols.Msg.GetPolicies()[0].GetEnabled())
	assert.Equal(t, approvalCedar, pols.Msg.GetPolicies()[0].GetCedar())
	_, err = h.DeleteCustomPolicy(e.ctx, connect.NewRequest(&werbotv1.DeleteCustomPolicyRequest{Id: pol.Msg.GetPolicy().GetId()}))
	require.NoError(t, err)

	_, err = h.DeleteCustomPolicy(e.ctx, connect.NewRequest(&werbotv1.DeleteCustomPolicyRequest{Id: "x"}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
