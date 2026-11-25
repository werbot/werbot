package access

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/werbot/werbot/internal/platform/apperr"
)

var (
	r1, r2     = uuid.New(), uuid.New()
	u1, u2, u3 = uuid.New(), uuid.New(), uuid.New()
	t1, t2     = uuid.New(), uuid.New()
)

func pol(name, mode string, mut func(*ApprovalPolicy)) ApprovalPolicy {
	p := ApprovalPolicy{ID: uuid.New(), Name: name, Mode: mode, MaxDuration: 8 * time.Hour,
		Selector:   Selector{Labels: map[string]string{"env": "prod"}},
		Requesters: Subjects{AllUsers: true}}
	if mut != nil {
		mut(&p)
	}
	return p
}

func TestSelectorMatches(t *testing.T) {
	byID := Selector{ResourceIDs: []uuid.UUID{r1}}
	byLabels := Selector{Labels: map[string]string{"env": "prod", "tier": "db"}}
	for name, c := range map[string]struct {
		s    Selector
		t    target
		want bool
	}{
		"id hit":          {byID, target{ID: r1}, true},
		"id miss":         {byID, target{ID: r2}, false},
		"labels superset": {byLabels, target{ID: r1, Labels: map[string]string{"env": "prod", "tier": "db", "x": "y"}}, true},
		"labels partial":  {byLabels, target{ID: r1, Labels: map[string]string{"env": "prod"}}, false},
		"label value":     {byLabels, target{ID: r1, Labels: map[string]string{"env": "dev", "tier": "db"}}, false},
		"empty selector":  {Selector{}, target{ID: r1, Labels: map[string]string{"env": "prod"}}, false},
	} {
		assert.Equal(t, c.want, c.s.matches(c.t), name)
	}
}

func TestSubjectsIncludes(t *testing.T) {
	human := requester{UserID: u1, TeamIDs: []uuid.UUID{t1}}
	agent := requester{UserID: u1, Agent: true}
	for name, c := range map[string]struct {
		s    Subjects
		r    requester
		want bool
	}{
		"user":                {Subjects{Users: []uuid.UUID{u1}}, human, true},
		"other user":          {Subjects{Users: []uuid.UUID{u2}}, human, false},
		"team":                {Subjects{Teams: []uuid.UUID{t2, t1}}, human, true},
		"other team":          {Subjects{Teams: []uuid.UUID{t2}}, human, false},
		"all users":           {Subjects{AllUsers: true}, human, true},
		"all users no agent":  {Subjects{AllUsers: true}, agent, false},
		"all agents":          {Subjects{AllAgents: true}, agent, true},
		"all agents no human": {Subjects{AllAgents: true}, human, false},
		"agent by user id":    {Subjects{Users: []uuid.UUID{u1}}, agent, false},
		"empty":               {Subjects{}, human, false},
	} {
		assert.Equal(t, c.want, c.s.includes(c.r), name)
	}
}

func TestMatchPoliciesBreakGlassOnly(t *testing.T) {
	plain := pol("a", ModeManual, nil)
	bg := pol("b", ModeManual, func(p *ApprovalPolicy) { p.BreakGlass = true })
	ts := []target{{ID: r1, Labels: map[string]string{"env": "prod"}}}
	r := requester{UserID: u1}
	got := matchPolicies([]ApprovalPolicy{bg, plain}, r, ts, false)
	assert.Equal(t, []string{"a", "b"}, names(got), "sorted by name, break-glass included")
	got = matchPolicies([]ApprovalPolicy{bg, plain}, r, ts, true)
	assert.Equal(t, []string{"b"}, names(got))
	assert.Empty(t, matchPolicies([]ApprovalPolicy{plain}, r, ts, true))
	assert.Empty(t, matchPolicies([]ApprovalPolicy{plain}, r, nil, false), "no targets")
	other := pol("c", ModeManual, func(p *ApprovalPolicy) { p.Requesters = Subjects{Users: []uuid.UUID{u2}} })
	assert.Empty(t, matchPolicies([]ApprovalPolicy{other}, r, ts, false), "requester not included")
	prod := pol("d", ModeManual, func(p *ApprovalPolicy) { p.Selector = Selector{Labels: map[string]string{"env": "dev"}} })
	assert.Empty(t, matchPolicies([]ApprovalPolicy{prod}, r, ts, false), "selector mismatch")
}

func names(ps []ApprovalPolicy) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Name)
	}
	return out
}

func TestSelectorRequestNeedsFullCoverage(t *testing.T) {
	ts := []target{{ID: r1, Labels: map[string]string{"env": "prod"}},
		{ID: r2, Labels: map[string]string{"env": "prod", "team": "x"}}}
	r := requester{UserID: u1}
	byID := pol("a", ModeManual, func(p *ApprovalPolicy) { p.Selector = Selector{ResourceIDs: []uuid.UUID{r1}} })
	assert.Empty(t, matchPolicies([]ApprovalPolicy{byID}, r, ts, false))
	byLabel := pol("b", ModeManual, nil)
	assert.Len(t, matchPolicies([]ApprovalPolicy{byLabel}, r, ts, false), 1)
}

func TestCombineManualWins(t *testing.T) {
	auto1 := pol("a", ModeAuto, func(p *ApprovalPolicy) { p.MaxDuration = 2 * time.Hour })
	auto2 := pol("b", ModeAuto, func(p *ApprovalPolicy) { p.MaxDuration = time.Hour })
	manual := pol("c", ModeManual, func(p *ApprovalPolicy) {
		p.MaxDuration = 4 * time.Hour
		p.RequireReason = true
	})
	manual2 := pol("d", ModeManual, func(p *ApprovalPolicy) { p.FourEyes = true })

	got := combine([]ApprovalPolicy{auto1, manual})
	assert.False(t, got.Auto)
	assert.Equal(t, manual.ID, got.PolicyID)
	assert.Equal(t, 2*time.Hour, got.MaxDuration)

	got = combine([]ApprovalPolicy{auto1, auto2})
	assert.True(t, got.Auto)
	assert.Equal(t, auto1.ID, got.PolicyID, "no manual: first")
	assert.Equal(t, time.Hour, got.MaxDuration)

	got = combine([]ApprovalPolicy{auto1, manual, manual2})
	assert.True(t, got.RequireReason)
	assert.True(t, got.FourEyes)
	assert.Equal(t, manual.ID, got.PolicyID)

	got = combine([]ApprovalPolicy{auto1})
	assert.False(t, got.RequireReason || got.FourEyes)
}

func TestCombineApprovers(t *testing.T) {
	m1 := pol("a", ModeManual, func(p *ApprovalPolicy) { p.Approvers = Subjects{Users: []uuid.UUID{u1}} })
	m2 := pol("b", ModeManual, func(p *ApprovalPolicy) { p.Approvers = Subjects{Teams: []uuid.UUID{t1}} })
	got := combine([]ApprovalPolicy{m1, m2})
	assert.Equal(t, []uuid.UUID{u1}, got.Approvers.Users)
	assert.Equal(t, []uuid.UUID{t1}, got.Approvers.Teams)
	assert.False(t, got.Admins)

	open := pol("c", ModeManual, nil)
	assert.True(t, combine([]ApprovalPolicy{m1, open}).Admins, "manual with empty approvers")

	auto := pol("d", ModeAuto, func(p *ApprovalPolicy) { p.Approvers = Subjects{Users: []uuid.UUID{u3}} })
	got = combine([]ApprovalPolicy{m1, auto})
	assert.Equal(t, []uuid.UUID{u1}, got.Approvers.Users, "auto approvers ignored")
	assert.False(t, combine([]ApprovalPolicy{m1, pol("e", ModeAuto, nil)}).Admins, "auto with empty approvers is not admins")
}

func TestApproversExcludeRequester(t *testing.T) {
	a := Approval{RequesterID: u1}
	admins := terms{Admins: true}
	assert.False(t, canDecide(a, admins, approver{UserID: u1, Role: "owner"}), "requester owner")
	assert.True(t, canDecide(a, admins, approver{UserID: u2, Role: "admin"}))
	assert.False(t, canDecide(a, admins, approver{UserID: u2, Role: "member"}))
	listed := terms{Approvers: Subjects{Users: []uuid.UUID{u1, u2}, Teams: []uuid.UUID{t1}}}
	assert.False(t, canDecide(a, listed, approver{UserID: u1, Role: "member"}), "requester in approvers")
	assert.True(t, canDecide(a, listed, approver{UserID: u2, Role: "member"}))
	assert.True(t, canDecide(a, listed, approver{UserID: u3, TeamIDs: []uuid.UUID{t1}, Role: "member"}), "by team")
	assert.False(t, canDecide(a, listed, approver{UserID: u3, TeamIDs: []uuid.UUID{t2}, Role: "member"}))
	assert.False(t, canDecide(a, listed, approver{UserID: u3, Role: "owner"}), "owner without Admins")
}

func TestFourEyesExcludesSponsor(t *testing.T) {
	run := uuid.New()
	cmd := Approval{RequesterID: u1, AgentRunID: &run}
	t.Run("sponsor without four eyes", func(t *testing.T) {
		assert.True(t, canDecide(cmd, terms{Admins: true}, approver{UserID: u1, Role: "owner"}))
	})
	t.Run("sponsor with four eyes", func(t *testing.T) {
		assert.False(t, canDecide(cmd, terms{Admins: true, FourEyes: true}, approver{UserID: u1, Role: "owner"}))
	})
	t.Run("access never self", func(t *testing.T) {
		acc := Approval{RequesterID: u1}
		assert.False(t, canDecide(acc, terms{Admins: true}, approver{UserID: u1, Role: "owner"}))
	})
}

func TestCommandTermsWithoutPolicies(t *testing.T) {
	got := commandTerms(nil)
	assert.True(t, got.Admins)
	assert.False(t, got.Auto)

	m := pol("a", ModeManual, func(p *ApprovalPolicy) { p.Approvers = Subjects{Users: []uuid.UUID{u1}} })
	got = commandTerms([]ApprovalPolicy{m})
	assert.Equal(t, []uuid.UUID{u1}, got.Approvers.Users)
	assert.False(t, got.Admins)
	assert.False(t, got.Auto)

	got = commandTerms([]ApprovalPolicy{pol("b", ModeAuto, nil)})
	assert.False(t, got.Auto)
	assert.True(t, got.Admins, "only auto policies: owners and admins approve the command")
}

func TestGrantDuration(t *testing.T) {
	const day = 24 * time.Hour
	def, maxTTL := 8*time.Hour, 14*day
	open := terms{MaxDuration: 24 * time.Hour}
	one := terms{MaxDuration: time.Hour}
	for name, c := range map[string]struct {
		req    time.Duration
		t      terms
		bg     bool
		want   time.Duration
		errMsg string
	}{
		"zero default":      {0, open, false, 8 * time.Hour, ""},
		"zero break-glass":  {0, open, true, time.Hour, ""},
		"zero clamped":      {0, one, false, time.Hour, ""},
		"explicit":          {2 * time.Hour, open, false, 2 * time.Hour, ""},
		"at limit":          {time.Hour, one, false, time.Hour, ""},
		"above policy":      {2 * time.Hour, one, false, 0, "duration exceeds 1h0m0s"},
		"above ttl":         {15 * day, terms{MaxDuration: 30 * day}, false, 0, "duration exceeds 336h0m0s"},
		"break-glass above": {2 * time.Hour, open, true, 0, "duration exceeds 1h0m0s"},
		"negative":          {-time.Second, open, false, 0, "duration must not be negative"},
	} {
		got, err := grantDuration(c.req, c.t, maxTTL, def, c.bg)
		if c.errMsg == "" {
			assert.NoError(t, err, name)
			assert.Equal(t, c.want, got, name)
			continue
		}
		assert.ErrorIs(t, err, apperr.ErrInvalidArgument, name)
		assert.ErrorContains(t, err, c.errMsg, name)
	}
}
