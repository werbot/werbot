package policy

import (
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	userID  = uuid.MustParse("019a0000-0000-7000-8000-000000000001")
	teamID  = uuid.MustParse("019a0000-0000-7000-8000-000000000002")
	resID   = uuid.MustParse("019a0000-0000-7000-8000-000000000003")
	bindID  = uuid.MustParse("019a0000-0000-7000-8000-000000000004")
	otherID = uuid.MustParse("019a0000-0000-7000-8000-000000000005")
)

// snap is one resource with a user binding on it and the given rules.
func snap(rules ...ResourceRules) Snapshot {
	return Snapshot{
		Resources: []SnapshotResource{{ID: resID, Name: "db", Type: "ssh", Labels: map[string]string{"env": "prod"}}},
		Bindings:  []SnapshotBinding{{ID: bindID, SubjectType: "user", SubjectID: userID, ResourceIDs: []uuid.UUID{resID}}},
		Rules:     rules,
	}
}

// req is a request of userID to resID; at defaults to Monday 2026-11-09 09:30 UTC.
func req(ip, country, at string) Request {
	if at == "" {
		at = "2026-11-09T09:30:00Z"
	}
	tm, err := time.Parse(time.RFC3339, at)
	if err != nil {
		panic(err)
	}
	return Request{UserID: userID, ResourceID: resID, Action: "ssh.connect", IP: netip.MustParseAddr(ip), Country: country, Time: tm}
}

func authorize(t *testing.T, s Snapshot, r Request) Decision {
	t.Helper()
	ps, _, err := Compile(s)
	require.NoError(t, err)
	return ps.Authorize(r)
}

func TestBindingUserAndTeam(t *testing.T) {
	d := authorize(t, snap(), req("10.1.2.3", "DE", ""))
	assert.True(t, d.Allow)
	assert.Equal(t, []string{"binding:" + bindID.String()}, d.Reasons)

	team := snap()
	team.Bindings[0].SubjectType, team.Bindings[0].SubjectID = "team", teamID
	r := req("10.1.2.3", "DE", "")
	r.UserID, r.TeamIDs = otherID, []uuid.UUID{teamID}
	assert.True(t, authorize(t, team, r).Allow)

	r = req("10.1.2.3", "DE", "")
	r.UserID = otherID
	d = authorize(t, snap(), r)
	assert.False(t, d.Allow)
	assert.Empty(t, d.Reasons)

	// A selector that matched nothing still compiles and grants nothing.
	empty := snap()
	empty.Bindings[0].ResourceIDs = nil
	assert.False(t, authorize(t, empty, req("10.1.2.3", "DE", "")).Allow)
}

func TestNetworkRules(t *testing.T) {
	s := snap(ResourceRules{ResourceID: resID, Networks: []NetworkRule{
		{CIDR: netip.MustParsePrefix("10.0.0.0/8"), Effect: EffectAllow},
		{CIDR: netip.MustParsePrefix("10.9.0.0/16"), Effect: EffectDeny},
	}})
	assert.True(t, authorize(t, s, req("10.1.2.3", "DE", "")).Allow)
	for _, ip := range []string{"10.9.1.1", "192.0.2.1"} {
		d := authorize(t, s, req(ip, "DE", ""))
		assert.False(t, d.Allow, ip)
		assert.Contains(t, d.Reasons, "rule:network:"+resID.String(), ip)
	}
}

func TestIPv4MappedAddressMatchesIPv4Rules(t *testing.T) {
	s := snap(ResourceRules{ResourceID: resID, Networks: []NetworkRule{
		{CIDR: netip.MustParsePrefix("10.0.0.0/8"), Effect: EffectAllow},
	}})
	assert.True(t, authorize(t, s, req("::ffff:10.1.2.3", "DE", "")).Allow)
}

func TestCountryRulesFailClosed(t *testing.T) {
	reason := "rule:country:" + resID.String()
	allowDE := snap(ResourceRules{ResourceID: resID, Countries: []CountryRule{{Country: "DE", Effect: EffectAllow}}})
	assert.True(t, authorize(t, allowDE, req("10.1.2.3", "DE", "")).Allow)
	for _, c := range []string{"FR", ""} {
		d := authorize(t, allowDE, req("10.1.2.3", c, ""))
		assert.False(t, d.Allow, c)
		assert.Contains(t, d.Reasons, reason, c)
	}

	denyCN := snap(ResourceRules{ResourceID: resID, Countries: []CountryRule{{Country: "CN", Effect: EffectDeny}}})
	assert.True(t, authorize(t, denyCN, req("10.1.2.3", "FR", "")).Allow)
	for _, c := range []string{"CN", ""} {
		d := authorize(t, denyCN, req("10.1.2.3", c, ""))
		assert.False(t, d.Allow, c)
		assert.Contains(t, d.Reasons, reason, c)
	}

	assert.True(t, authorize(t, snap(), req("10.1.2.3", "", "")).Allow)
}

func TestScheduleUsesUTCHourBoundaries(t *testing.T) {
	s := snap(ResourceRules{ResourceID: resID, Schedule: []Slot{{Weekday: 1, Hour: 9}}})
	for at, allow := range map[string]bool{
		"2026-11-09T09:59:59Z":      true,
		"2026-11-09T10:00:00Z":      false,
		"2026-11-09T12:30:00+03:00": true,
		"2026-11-08T09:30:00Z":      false,
	} {
		d := authorize(t, s, req("10.1.2.3", "DE", at))
		assert.Equal(t, allow, d.Allow, at)
		if !allow {
			assert.Contains(t, d.Reasons, "rule:schedule:"+resID.String(), at)
		}
	}
}

func TestBlocklistDenies(t *testing.T) {
	r := req("10.1.2.3", "DE", "")
	r.IPBlocked = true
	d := authorize(t, snap(), r)
	assert.False(t, d.Allow)
	assert.Equal(t, []string{"blocklist"}, d.Reasons)
}

func TestInvalidIPDenies(t *testing.T) {
	r := req("10.1.2.3", "DE", "")
	r.IP = netip.Addr{}
	assert.Equal(t, Decision{Allow: false, Reasons: []string{"invalid_ip"}}, authorize(t, snap(), r))
}

func TestEvaluationErrorDenies(t *testing.T) {
	policies, err := parse(`@id("ok") permit (principal, action, resource);
@id("bad") forbid (principal, action, resource) when { context.missing };`)
	require.NoError(t, err)
	d := (&PolicySet{policies: policies}).Authorize(req("10.1.2.3", "DE", ""))
	assert.False(t, d.Allow)
	assert.Equal(t, []string{"ok", "error:bad"}, d.Reasons)
}

func TestCompileRejectsInvalidInput(t *testing.T) {
	bad := map[string]Snapshot{
		"effect":       snap(ResourceRules{ResourceID: resID, Networks: []NetworkRule{{CIDR: netip.MustParsePrefix("10.0.0.0/8"), Effect: "maybe"}}}),
		"country":      snap(ResourceRules{ResourceID: resID, Countries: []CountryRule{{Country: `D"E`, Effect: EffectAllow}}}),
		"cidr":         snap(ResourceRules{ResourceID: resID, Networks: []NetworkRule{{Effect: EffectAllow}}}),
		"subject":      {Bindings: []SnapshotBinding{{ID: bindID, SubjectType: "robot", SubjectID: userID}}},
		"duplicate id": snap(ResourceRules{ResourceID: resID, Schedule: []Slot{{1, 9}}}, ResourceRules{ResourceID: resID, Schedule: []Slot{{2, 9}}}),
	}
	for name, s := range bad {
		ps, _, err := Compile(s)
		assert.Error(t, err, name)
		assert.Nil(t, ps, name)
	}
}

func TestCompileDeterministicIDs(t *testing.T) {
	r2 := uuid.MustParse("019a0000-0000-7000-8000-000000000006")
	b2 := uuid.MustParse("019a0000-0000-7000-8000-000000000007")
	s := snap(
		ResourceRules{ResourceID: r2, Countries: []CountryRule{{"FR", EffectAllow}, {"DE", EffectAllow}}},
		ResourceRules{ResourceID: resID, Networks: []NetworkRule{
			{CIDR: netip.MustParsePrefix("192.168.0.0/16"), Effect: EffectAllow},
			{CIDR: netip.MustParsePrefix("10.0.0.0/8"), Effect: EffectAllow},
		}},
	)
	s.Bindings = append(s.Bindings, SnapshotBinding{ID: b2, SubjectType: "team", SubjectID: teamID, ResourceIDs: []uuid.UUID{r2, resID}})
	_, text1, err := Compile(s)
	require.NoError(t, err)

	// Reversed input order must produce the same text.
	s.Bindings[0], s.Bindings[1] = s.Bindings[1], s.Bindings[0]
	s.Rules[0], s.Rules[1] = s.Rules[1], s.Rules[0]
	s.Bindings[0].ResourceIDs = []uuid.UUID{resID, r2}
	_, text2, err := Compile(s)
	require.NoError(t, err)

	assert.Equal(t, text1, text2)
	assert.Contains(t, text1, `@id("binding:`+bindID.String()+`")`)
	assert.Contains(t, text1, `@id("rule:network:`+resID.String()+`")`)
	assert.Contains(t, text1, `@id("blocklist")`)
}

func TestExpandBindings(t *testing.T) {
	r1 := SnapshotResource{ID: uuid.MustParse("019a0000-0000-7000-8000-0000000000a1"), Labels: map[string]string{"env": "prod", "tier": "db"}}
	r2 := SnapshotResource{ID: uuid.MustParse("019a0000-0000-7000-8000-0000000000a2"), Labels: map[string]string{"env": "prod"}}
	r3 := SnapshotResource{ID: uuid.MustParse("019a0000-0000-7000-8000-0000000000a3"), Labels: map[string]string{"env": "dev"}}
	got := ExpandBindings([]SnapshotResource{r1, r2, r3}, []Binding{
		{ID: bindID, SubjectType: "user", SubjectID: userID, LabelSelector: map[string]string{"env": "prod"}},
		{ID: otherID, SubjectType: "team", SubjectID: teamID, LabelSelector: map[string]string{"env": "prod", "tier": "db"}},
		{ID: teamID, SubjectType: "user", SubjectID: userID, ResourceID: &r3.ID},
	})
	require.Len(t, got, 3)
	assert.Equal(t, SnapshotBinding{ID: bindID, SubjectType: "user", SubjectID: userID, ResourceIDs: []uuid.UUID{r1.ID, r2.ID}}, got[0])
	assert.Equal(t, []uuid.UUID{r1.ID}, got[1].ResourceIDs)
	assert.Equal(t, []uuid.UUID{r3.ID}, got[2].ResourceIDs)
}
