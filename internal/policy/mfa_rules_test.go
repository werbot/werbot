package policy

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

func prodMFA(r uuid.UUID) MFARuleInput {
	return MFARuleInput{Name: "prod", Subjects: Subjects{AllUsers: true}, Selector: Selector{ResourceIDs: []uuid.UUID{r}},
		MaxAge: 5 * time.Minute, Strength: "any"}
}

func TestMFARuleCRUD(t *testing.T) {
	e := setup(t)
	r := e.resource(t, e.org)

	rule, err := e.svc.CreateMFARule(e.ctx, e.org, prodMFA(r))
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, rule.ID)
	assert.Equal(t, prodMFA(r), rule.MFARuleInput)
	assert.False(t, rule.CreatedAt.IsZero())

	list, err := e.svc.ListMFARules(e.ctx, e.org)
	require.NoError(t, err)
	assert.Equal(t, []MFARule{rule}, list)

	_, err = e.svc.CreateMFARule(e.ctx, e.org, prodMFA(r))
	assert.ErrorIs(t, err, apperr.ErrAlreadyExists)

	in := prodMFA(r)
	in.Name, in.MaxAge, in.Strength, in.Subjects = "prod-keys", 24*time.Hour, "webauthn", Subjects{Users: []uuid.UUID{e.owner}}
	upd, err := e.svc.UpdateMFARule(e.ctx, e.org, rule.ID, in)
	require.NoError(t, err)
	assert.Equal(t, in, upd.MFARuleInput)
	assert.Equal(t, rule.ID, upd.ID)
	_, err = e.svc.UpdateMFARule(e.ctx, e.org, uuid.New(), in)
	assert.ErrorIs(t, err, apperr.ErrNotFound)

	snap, err := e.svc.Snapshot(e.ctx, e.org)
	require.NoError(t, err)
	assert.Equal(t, []SnapshotMFARule{{ID: rule.ID, Subjects: in.Subjects, ResourceIDs: []uuid.UUID{r}, MaxAgeSeconds: 86400, Strength: "webauthn"}},
		snap.MFARules)

	require.NoError(t, e.svc.DeleteMFARule(e.ctx, e.org, rule.ID))
	assert.ErrorIs(t, e.svc.DeleteMFARule(e.ctx, e.org, rule.ID), apperr.ErrNotFound)
	list, err = e.svc.ListMFARules(e.ctx, e.org)
	require.NoError(t, err)
	assert.Empty(t, list)

	assert.Equal(t, []string{`{"name": "prod", "strength": "any", "max_age_seconds": 300}`}, auditData(t, e, "mfa_rule.create"))
	updated := `{"name": "prod-keys", "strength": "webauthn", "max_age_seconds": 86400}`
	assert.Equal(t, []string{updated}, auditData(t, e, "mfa_rule.update"))
	assert.Equal(t, []string{updated}, auditData(t, e, "mfa_rule.delete"))
}

// TestMFARuleRejects: max_age outside 30s–24h or not whole seconds, an
// unknown strength, agent subjects and references of another organization
// are refused.
func TestMFARuleRejects(t *testing.T) {
	e := setup(t)
	r := e.resource(t, e.org)
	orgB := dbtest.Org(t, e.pool)
	userB := dbtest.User(t, e.pool, orgB, "member")
	resB := e.resource(t, orgB)
	for name, mut := range map[string]func(*MFARuleInput){
		"max_age 29s":       func(in *MFARuleInput) { in.MaxAge = 29 * time.Second },
		"max_age 25h":       func(in *MFARuleInput) { in.MaxAge = 25 * time.Hour },
		"max_age fraction":  func(in *MFARuleInput) { in.MaxAge = 90*time.Second + time.Millisecond },
		"strength sms":      func(in *MFARuleInput) { in.Strength = "sms" },
		"all agents":        func(in *MFARuleInput) { in.Subjects = Subjects{AllAgents: true} },
		"users with agents": func(in *MFARuleInput) { in.Subjects = Subjects{Users: []uuid.UUID{e.owner}, AllAgents: true} },
		"empty name":        func(in *MFARuleInput) { in.Name = "" },
		"empty subjects":    func(in *MFARuleInput) { in.Subjects = Subjects{} },
		"other org user":    func(in *MFARuleInput) { in.Subjects = Subjects{Users: []uuid.UUID{userB}} },
		"other org res":     func(in *MFARuleInput) { in.Selector.ResourceIDs = []uuid.UUID{resB} },
	} {
		in := prodMFA(r)
		mut(&in)
		_, err := e.svc.CreateMFARule(e.ctx, e.org, in)
		assert.ErrorIs(t, err, apperr.ErrInvalidArgument, name)
	}
	in := prodMFA(r)
	in.Subjects = Subjects{AllAgents: true}
	_, err := e.svc.CreateMFARule(e.ctx, e.org, in)
	assert.ErrorContains(t, err, "mfa rules apply to users only")
	for _, d := range []time.Duration{30 * time.Second, 24 * time.Hour} {
		in := prodMFA(r)
		in.Name, in.MaxAge = d.String(), d
		_, err := e.svc.CreateMFARule(e.ctx, e.org, in)
		assert.NoError(t, err, d)
	}
}

func TestMFARuleRPC(t *testing.T) {
	e := setup(t)
	h := NewHandler(e.svc)
	member := dbtest.User(t, e.pool, e.org, "member")
	mctx := principal.With(context.Background(), principal.Principal{UserID: member, OrgID: e.org, Role: "member"})
	for name, call := range map[string]func(context.Context) error{
		"ListMFARules": func(c context.Context) error {
			_, err := h.ListMFARules(c, connect.NewRequest(&werbotv1.ListMFARulesRequest{}))
			return err
		},
		"CreateMFARule": func(c context.Context) error {
			_, err := h.CreateMFARule(c, connect.NewRequest(&werbotv1.CreateMFARuleRequest{}))
			return err
		},
		"UpdateMFARule": func(c context.Context) error {
			_, err := h.UpdateMFARule(c, connect.NewRequest(&werbotv1.UpdateMFARuleRequest{}))
			return err
		},
		"DeleteMFARule": func(c context.Context) error {
			_, err := h.DeleteMFARule(c, connect.NewRequest(&werbotv1.DeleteMFARuleRequest{}))
			return err
		},
	} {
		assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(call(mctx)), name)
	}

	r := e.resource(t, e.org)
	subj, sel := &werbotv1.Subjects{AllUsers: true}, &werbotv1.ResourceSelector{ResourceIds: []string{r.String()}}
	created, err := h.CreateMFARule(e.ctx, connect.NewRequest(&werbotv1.CreateMFARuleRequest{Name: "prod", Subjects: subj,
		ResourceSelector: sel, MaxAge: durationpb.New(90 * time.Second), Strength: "webauthn"}))
	require.NoError(t, err)
	rule := created.Msg.GetRule()
	assert.Equal(t, 90*time.Second, rule.GetMaxAge().AsDuration())
	assert.Equal(t, "webauthn", rule.GetStrength())
	assert.Equal(t, []string{r.String()}, rule.GetResourceSelector().GetResourceIds())
	assert.True(t, rule.GetSubjects().GetAllUsers())
	assert.NotNil(t, rule.GetCreatedAt())

	_, err = h.CreateMFARule(e.ctx, connect.NewRequest(&werbotv1.CreateMFARuleRequest{Name: "agents", Subjects: &werbotv1.Subjects{AllAgents: true},
		ResourceSelector: sel, MaxAge: durationpb.New(time.Minute), Strength: "any"}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	upd, err := h.UpdateMFARule(e.ctx, connect.NewRequest(&werbotv1.UpdateMFARuleRequest{Id: rule.GetId(), Name: "prod", Subjects: subj,
		ResourceSelector: sel, MaxAge: durationpb.New(time.Hour), Strength: "any"}))
	require.NoError(t, err)
	assert.Equal(t, time.Hour, upd.Msg.GetRule().GetMaxAge().AsDuration())
	list, err := h.ListMFARules(e.ctx, connect.NewRequest(&werbotv1.ListMFARulesRequest{}))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetRules(), 1)
	assert.Equal(t, "any", list.Msg.GetRules()[0].GetStrength())
	_, err = h.DeleteMFARule(e.ctx, connect.NewRequest(&werbotv1.DeleteMFARuleRequest{Id: rule.GetId()}))
	require.NoError(t, err)
}

// TestAccessibleResourcesIncludeMFA: a resource a person reaches after an
// MFA check is listed (inventory marks it allowed, so werbot ssh goes on);
// one another rule denies is not.
func TestAccessibleResourcesIncludeMFA(t *testing.T) {
	e := setup(t)
	r1, r2 := e.resource(t, e.org), e.resource(t, e.org)
	e.bind(e.org, e.owner, r1)
	e.bind(e.org, e.owner, r2)
	in := prodMFA(r1)
	in.Selector.ResourceIDs = []uuid.UUID{r1, r2}
	_, err := e.svc.CreateMFARule(e.ctx, e.org, in)
	require.NoError(t, err)
	require.NoError(t, e.svc.SetRules(e.ctx, e.org, ResourceRules{ResourceID: r2,
		Networks: []NetworkRule{{CIDR: netip.MustParsePrefix("10.0.0.0/8"), Effect: EffectDeny}}}))

	got, err := e.svc.AccessibleResources(e.ctx, e.org, e.owner, ip10, time.Time{})
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{r1}, got)
}

// TestAgentResourcesExcludeMFA: an agent of a sponsor under an MFA rule
// never lists the resource; the sponsor does.
func TestAgentResourcesExcludeMFA(t *testing.T) {
	e, a1 := agentSetup(t)
	r := e.resource(t, e.org)
	e.bind(e.org, e.owner, r)
	agent, err := e.svc.AgentResources(e.ctx, e.org, a1, ip10, time.Time{})
	require.NoError(t, err)
	require.Contains(t, agent, r, "listed without the rule")

	_, err = e.svc.CreateMFARule(e.ctx, e.org, prodMFA(r))
	require.NoError(t, err)
	agent, err = e.svc.AgentResources(e.ctx, e.org, a1, ip10, time.Time{})
	require.NoError(t, err)
	assert.NotContains(t, agent, r)
	person, err := e.svc.AccessibleResources(e.ctx, e.org, e.owner, ip10, time.Time{})
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{r}, person)
}
