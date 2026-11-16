package gateway

import (
	"encoding/json"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/cedar-policy/cedar-go"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/connector"
	_ "github.com/werbot/werbot/internal/connector/ssh" // registers "ssh"
)

func init() {
	// a registered type without a connect action
	connector.Register(connector.Type{Name: "other", NewConfig: func() proto.Message { return &emptypb.Empty{} },
		Validate: func(proto.Message) error { return nil }, Actions: []string{"other.exec"}, Kinds: []connector.Kind{connector.KindExec}})
}

// testPolicies is the contract §3.2 shape; cedar has no "resource in [list]" scope, so r2–r4 use contains.
const testPolicies = `
permit (principal == Werbot::User::"u1", action == Werbot::Action::"ssh.connect", resource == Werbot::Resource::"r1");
permit (principal in Werbot::Team::"t1", action == Werbot::Action::"ssh.connect", resource) when { [Werbot::Resource::"r2", Werbot::Resource::"r3", Werbot::Resource::"r4"].contains(resource) };
forbid (principal, action, resource == Werbot::Resource::"r1") when { context.ip.isInRange(ip("10.0.0.0/8")) };
forbid (principal, action, resource == Werbot::Resource::"r3") unless { ["NL"].contains(context.country) };
forbid (principal, action, resource == Werbot::Resource::"r4") unless { context.weekday == 1 && context.hour == 10 };
forbid (principal, action, resource) when { context.ip_blocked };`

// ids maps the short names of test policies to uuids.
type ids map[string]uuid.UUID

func newIDs(names ...string) ids {
	m := ids{}
	for _, n := range names {
		m[n] = uuid.New()
	}
	return m
}

// expand replaces every quoted short name in text with its uuid.
func (m ids) expand(text string) string {
	var pairs []string
	for n, id := range m {
		pairs = append(pairs, `"`+n+`"`, `"`+id.String()+`"`)
	}
	return strings.NewReplacer(pairs...).Replace(text)
}

// res is a snapshot resource: id name, display name, type.
type res struct{ id, name, typ string }

// compileTest builds a snapshot the way the server does: cedar text plus the resource entities.
func compileTest(t *testing.T, m ids, policies string, blocklist []string, rs ...res) *Policy {
	t.Helper()
	ents := cedar.EntityMap{}
	s := &gatewayv1.Snapshot{CedarPolicies: m.expand(policies), Blocklist: blocklist, Version: "v1"}
	for _, r := range rs {
		uid := cedar.NewEntityUID("Werbot::Resource", cedar.String(m[r.id].String()))
		ents[uid] = cedar.Entity{UID: uid, Attributes: cedar.NewRecord(cedar.RecordMap{"name": cedar.String(r.name), "type": cedar.String(r.typ)})}
		s.Resources = append(s.Resources, &gatewayv1.Resource{Id: m[r.id].String(), Name: r.name, Type: r.typ, ConfigJson: []byte(`{}`)})
	}
	var err error
	s.EntitiesJson, err = json.Marshal(ents)
	require.NoError(t, err)
	p, err := CompilePolicy(s)
	require.NoError(t, err)
	return p
}

func TestAuthorize(t *testing.T) {
	m := newIDs("u1", "u2", "t1", "r1", "r2", "r3", "r4")
	rs := []res{{"r1", "r1", "ssh"}, {"r2", "r2", "ssh"}, {"r3", "r3", "ssh"}, {"r4", "r4", "ssh"}}
	p := compileTest(t, m, testPolicies, nil, rs...)
	q := compileTest(t, m, testPolicies, []string{"192.0.2.0/24"}, rs...)
	u1 := User{ID: m["u1"]}
	u2 := User{ID: m["u2"], TeamIDs: []uuid.UUID{m["t1"]}}
	r := func(n string) Resource { got, ok := p.Resource(n); require.True(t, ok, n); return got }
	public, private := netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("10.1.2.3")
	mon := time.Date(2026, 11, 2, 10, 30, 0, 0, time.UTC)
	msk := time.FixedZone("UTC+3", 3*3600)

	for _, tc := range []struct {
		name string
		p    *Policy
		req  AccessRequest
		want bool
	}{
		{"user binding", p, AccessRequest{User: u1, Resource: r("r1"), IP: public}, true},
		{"cidr forbid", p, AccessRequest{User: u1, Resource: r("r1"), IP: private}, false},
		{"team binding", p, AccessRequest{User: u2, Resource: r("r2"), IP: public}, true},
		{"no team", p, AccessRequest{User: User{ID: m["u2"]}, Resource: r("r2"), IP: public}, false},
		{"country NL", p, AccessRequest{User: User{ID: m["u2"], TeamIDs: u2.TeamIDs, Country: "NL"}, Resource: r("r3"), IP: public}, true},
		{"country US", p, AccessRequest{User: User{ID: m["u2"], TeamIDs: u2.TeamIDs, Country: "US"}, Resource: r("r3"), IP: public}, false},
		{"country unknown", p, AccessRequest{User: u2, Resource: r("r3"), IP: public}, false},
		{"schedule hit", p, AccessRequest{User: u2, Resource: r("r4"), IP: public, Time: mon}, true},
		{"schedule miss", p, AccessRequest{User: u2, Resource: r("r4"), IP: public, Time: mon.Add(time.Hour)}, false},
		{"schedule in UTC+3", p, AccessRequest{User: u2, Resource: r("r4"), IP: public, Time: mon.In(msk)}, true},
		{"local hour ignored", p, AccessRequest{User: u2, Resource: r("r4"), IP: public, Time: time.Date(2026, 11, 2, 10, 30, 0, 0, msk)}, false},
		{"blocklist", q, AccessRequest{User: u1, Resource: r("r1"), IP: public}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			allow, reasons := tc.p.Authorize(tc.req)
			require.Equal(t, tc.want, allow, reasons)
		})
	}
}

func TestAuthorizeFailClosed(t *testing.T) {
	m := newIDs("u1", "r1")
	permit := `permit (principal == Werbot::User::"u1", action, resource) when { [Werbot::Resource::"r1"].contains(resource) };
forbid (principal, action, resource) when { context.ip_blocked };`
	p := compileTest(t, m, permit, []string{"192.0.2.0/24"}, res{"r1", "r1", "ssh"})
	r1, _ := p.Resource("r1")
	u1 := User{ID: m["u1"]}

	allow, _ := p.Authorize(AccessRequest{User: u1, Resource: r1, IP: netip.MustParseAddr("198.51.100.1")})
	require.True(t, allow)
	allow, reasons := p.Authorize(AccessRequest{User: u1, Resource: r1})
	require.False(t, allow)
	require.Equal(t, []string{"invalid_ip"}, reasons)
	allow, _ = p.Authorize(AccessRequest{User: u1, Resource: r1, IP: netip.MustParseAddr("::ffff:192.0.2.1")}) // mapped, blocklisted
	require.False(t, allow)

	broken := compileTest(t, m, permit+"\nforbid (principal, action, resource) when { context.missing == 1 };", nil, res{"r1", "r1", "ssh"})
	allow, reasons = broken.Authorize(AccessRequest{User: u1, Resource: r1, IP: netip.MustParseAddr("198.51.100.1")})
	require.False(t, allow, "a forbid that fails to evaluate must deny")
	require.Contains(t, reasons, "error:policy2")

	var none *Policy
	allow, _ = none.Authorize(AccessRequest{User: u1, Resource: r1, IP: netip.MustParseAddr("198.51.100.1")})
	require.False(t, allow)
}

func TestAuthorizeUsesTypeAction(t *testing.T) {
	m := newIDs("u1", "r1", "r2")
	// compiled shape: a binding permits every action
	bind := `permit (principal == Werbot::User::"u1", action, resource) when { [Werbot::Resource::"r1", Werbot::Resource::"r2"].contains(resource) };`
	p := compileTest(t, m, bind, nil, res{"r1", "web", "ssh"}, res{"r2", "other", "other"})
	u1 := User{ID: m["u1"]}
	ip := netip.MustParseAddr("198.51.100.1")
	web, _ := p.Resource("web")
	other, _ := p.Resource("other")

	allow, _ := p.Authorize(AccessRequest{User: u1, Resource: web, IP: ip})
	require.True(t, allow)
	allow, reasons := p.Authorize(AccessRequest{User: u1, Resource: other, IP: ip}) // "other.connect" is not an action of "other"
	require.False(t, allow)
	require.Equal(t, []string{"invalid_action"}, reasons)
	web.Type = "other" // the request type must match the snapshot entity
	allow, _ = p.Authorize(AccessRequest{User: u1, Resource: web, IP: ip})
	require.False(t, allow)
}

func TestPermittedSorted(t *testing.T) {
	m := newIDs("u1", "web", "db", "denied")
	bind := `permit (principal == Werbot::User::"u1", action, resource) when { [Werbot::Resource::"web", Werbot::Resource::"db"].contains(resource) };`
	p := compileTest(t, m, bind, nil, res{"web", "web", "ssh"}, res{"db", "db", "ssh"}, res{"denied", "r-denied", "ssh"})
	got := p.Permitted(User{ID: m["u1"]}, netip.MustParseAddr("198.51.100.1"), time.Now())
	require.Len(t, got, 2)
	require.Equal(t, "db", got[0].Name)
	require.Equal(t, "web", got[1].Name)
	require.Equal(t, m["db"], got[0].ID)
	require.Equal(t, []byte(`{}`), got[0].Config)
	require.Equal(t, "v1", p.Version())
	_, ok := p.Resource("missing")
	require.False(t, ok)
}

func TestCompilePolicyRejectsGarbage(t *testing.T) {
	for name, s := range map[string]*gatewayv1.Snapshot{
		"cedar":       {CedarPolicies: "permit ("},
		"entities":    {EntitiesJson: []byte("{")},
		"blocklist":   {Blocklist: []string{"not-a-cidr"}},
		"resource id": {Resources: []*gatewayv1.Resource{{Id: "x", Name: "a", Type: "ssh"}}},
		"dup name":    {Resources: []*gatewayv1.Resource{{Id: uuid.NewString(), Name: "a"}, {Id: uuid.NewString(), Name: "a"}}},
	} {
		_, err := CompilePolicy(s)
		require.Error(t, err, name)
	}

	// the server's deny-all snapshot: empty text and entities
	p, err := CompilePolicy(&gatewayv1.Snapshot{Resources: []*gatewayv1.Resource{{Id: uuid.NewString(), Name: "a", Type: "ssh"}}})
	require.NoError(t, err)
	require.Empty(t, p.Permitted(User{ID: uuid.New()}, netip.MustParseAddr("198.51.100.1"), time.Now()))
}
