package policy

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/connector"
	_ "github.com/werbot/werbot/internal/connector/ssh"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/policy/geoiptest"
)

type fakeResources map[uuid.UUID][]SnapshotResource

func (f fakeResources) PolicyResources(_ context.Context, orgID uuid.UUID) ([]SnapshotResource, error) {
	return f[orgID], nil
}

type fakeBindings map[uuid.UUID][]Binding

func (f fakeBindings) PolicyBindings(_ context.Context, orgID uuid.UUID) ([]Binding, error) {
	return f[orgID], nil
}

type noTeams struct{}

func (noTeams) UserTeams(context.Context, uuid.UUID, uuid.UUID) ([]uuid.UUID, error) { return nil, nil }

type env struct {
	svc   *Service
	pool  *pgxpool.Pool
	ctx   context.Context
	org   uuid.UUID
	owner uuid.UUID
	res   fakeResources
	binds fakeBindings
}

func setup(t *testing.T) env {
	t.Helper()
	pool, _ := dbtest.New(t)
	org := dbtest.Org(t, pool)
	owner := dbtest.User(t, pool, org, "owner")
	ctx := principal.With(context.Background(), principal.Principal{UserID: owner, OrgID: org, Role: "owner"})
	e := env{pool: pool, ctx: ctx, org: org, owner: owner, res: fakeResources{}, binds: fakeBindings{}}
	e.svc = New(Deps{Pool: pool, Resources: e.res, Bindings: e.binds, Teams: noTeams{}, Logger: slog.New(slog.DiscardHandler)})
	return e
}

// resource inserts an ssh resource of org and lists it in the fake source.
func (e env) resource(t *testing.T, org uuid.UUID) uuid.UUID {
	t.Helper()
	name := fmt.Sprintf("r%d", len(e.res[org]))
	id := dbtest.Resource(t, e.pool, org, name, nil)
	e.res[org] = append(e.res[org], SnapshotResource{ID: id, Name: name, Type: "ssh"})
	return id
}

func (e env) bind(org, user, resource uuid.UUID) {
	e.binds[org] = append(e.binds[org], Binding{ID: uuid.New(), SubjectType: "user", SubjectID: user, ResourceID: &resource})
}

var ip10 = netip.MustParseAddr("10.1.2.3")

func TestSetGetRules(t *testing.T) {
	e := setup(t)
	r := e.resource(t, e.org)
	require.NoError(t, e.svc.SetRules(e.ctx, e.org, ResourceRules{ResourceID: r,
		Networks:  []NetworkRule{{CIDR: netip.MustParsePrefix("10.0.0.1/8"), Effect: EffectAllow}},
		Countries: []CountryRule{{Country: "DE", Effect: EffectAllow}},
		Schedule:  []Slot{{Weekday: 1, Hour: 9}},
	}))
	got, err := e.svc.GetRules(e.ctx, e.org, r)
	require.NoError(t, err)
	assert.Equal(t, ResourceRules{ResourceID: r,
		Networks:  []NetworkRule{{CIDR: netip.MustParsePrefix("10.0.0.0/8"), Effect: EffectAllow}},
		Countries: []CountryRule{{Country: "DE", Effect: EffectAllow}},
		Schedule:  []Slot{{Weekday: 1, Hour: 9}},
	}, got)

	var n int
	require.NoError(t, e.pool.QueryRow(context.Background(),
		"SELECT count(*) FROM audit_event WHERE org_id = $1 AND action = 'policy.rules.set' AND target_id = $2", e.org, r.String()).Scan(&n))
	assert.Equal(t, 1, n)

	// The three sets are replaced: an empty set clears them.
	require.NoError(t, e.svc.SetRules(e.ctx, e.org, ResourceRules{ResourceID: r}))
	got, err = e.svc.GetRules(e.ctx, e.org, r)
	require.NoError(t, err)
	assert.Equal(t, ResourceRules{ResourceID: r}, got)
}

func TestSetRulesRejects(t *testing.T) {
	e := setup(t)
	r := e.resource(t, e.org)
	allow := func(cidr string) NetworkRule {
		return NetworkRule{CIDR: netip.MustParsePrefix(cidr), Effect: EffectAllow}
	}
	for name, rules := range map[string]ResourceRules{
		"cidr":           {Networks: []NetworkRule{{Effect: EffectAllow}}}, // what "x" parses to
		"mapped cidr":    {Networks: []NetworkRule{allow("::ffff:10.0.0.0/104")}},
		"effect":         {Networks: []NetworkRule{{CIDR: netip.MustParsePrefix("10.0.0.0/8"), Effect: "maybe"}}},
		"country":        {Countries: []CountryRule{{Country: "de", Effect: EffectAllow}}},
		"weekday":        {Schedule: []Slot{{Weekday: 7, Hour: 0}}},
		"hour":           {Schedule: []Slot{{Weekday: 0, Hour: 24}}},
		"dup cidr":       {Networks: []NetworkRule{allow("10.0.0.0/8"), allow("10.1.0.0/8")}},
		"dup country":    {Countries: []CountryRule{{"DE", EffectAllow}, {"DE", EffectDeny}}},
		"dup slot":       {Schedule: []Slot{{1, 9}, {1, 9}}},
		"negative hour":  {Schedule: []Slot{{Weekday: 0, Hour: -1}}},
		"unknown effect": {Countries: []CountryRule{{Country: "DE"}}},
	} {
		rules.ResourceID = r
		err := e.svc.SetRules(e.ctx, e.org, rules)
		assert.ErrorIs(t, err, apperr.ErrInvalidArgument, name)
	}
}

func TestOwnerWithoutBindingIsDenied(t *testing.T) {
	e := setup(t)
	r := e.resource(t, e.org)
	d, err := e.svc.CheckAccess(e.ctx, e.org, e.owner, r, ip10, time.Time{}, 0)
	require.NoError(t, err)
	assert.False(t, d.Allow)

	e.bind(e.org, e.owner, r)
	d, err = e.svc.CheckAccess(e.ctx, e.org, e.owner, r, ip10, time.Time{}, 0)
	require.NoError(t, err)
	assert.True(t, d.Allow)
}

func TestCheckAccessFailsClosedWithoutGeoIP(t *testing.T) {
	e := setup(t)
	r := e.resource(t, e.org)
	e.bind(e.org, e.owner, r)
	require.NoError(t, e.svc.SetRules(e.ctx, e.org, ResourceRules{ResourceID: r, Countries: []CountryRule{{"DE", EffectAllow}}}))

	require.NoError(t, e.svc.ReloadGeoIP(e.ctx)) // no geoip_db row
	d, err := e.svc.CheckAccess(e.ctx, e.org, e.owner, r, ip10, time.Time{}, 0)
	require.NoError(t, err)
	assert.False(t, d.Allow)
	assert.Contains(t, d.Reasons, "rule:country:"+r.String())

	_, err = e.pool.Exec(context.Background(), "INSERT INTO geoip_db (id, month, data) VALUES (1, '2026-11-01', $1)",
		geoiptest.MMDB(netip.MustParsePrefix("10.0.0.0/8"), "DE"))
	require.NoError(t, err)
	require.NoError(t, e.svc.ReloadGeoIP(e.ctx))
	d, err = e.svc.CheckAccess(e.ctx, e.org, e.owner, r, ip10, time.Time{}, 0)
	require.NoError(t, err)
	assert.True(t, d.Allow)
}

func TestCheckAccessBlocklisted(t *testing.T) {
	e := setup(t)
	r := e.resource(t, e.org)
	e.bind(e.org, e.owner, r)
	_, err := e.pool.Exec(context.Background(), "INSERT INTO ip_blocklist (cidr, source) VALUES ('203.0.113.0/24', 'test')")
	require.NoError(t, err)

	for _, ip := range []string{"203.0.113.7", "::ffff:203.0.113.7"} {
		d, err := e.svc.CheckAccess(e.ctx, e.org, e.owner, r, netip.MustParseAddr(ip), time.Time{}, 0)
		require.NoError(t, err, ip)
		assert.Equal(t, Decision{Allow: false, Reasons: []string{"blocklist"}}, d, ip)
	}
	list, err := e.svc.Blocklist(e.ctx)
	require.NoError(t, err)
	assert.Equal(t, []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")}, list)
}

func TestCheckAccessOtherOrgResource(t *testing.T) {
	e := setup(t)
	r := e.resource(t, e.org)
	orgB := dbtest.Org(t, e.pool)
	e.resource(t, orgB)
	e.bind(orgB, e.owner, r)

	_, err := e.svc.CheckAccess(e.ctx, orgB, e.owner, r, ip10, time.Time{}, 0)
	assert.ErrorIs(t, err, apperr.ErrNotFound)
	_, err = e.svc.GetRules(e.ctx, orgB, r)
	assert.ErrorIs(t, err, apperr.ErrNotFound)
	err = e.svc.SetRules(e.ctx, orgB, ResourceRules{ResourceID: r})
	assert.ErrorIs(t, err, apperr.ErrNotFound)
}

func TestCheckAccessCompileErrorDenies(t *testing.T) {
	e := setup(t)
	r := e.resource(t, e.org)
	e.binds[e.org] = []Binding{{ID: uuid.New(), SubjectType: "robot", SubjectID: e.owner, ResourceID: &r}}
	d, err := e.svc.CheckAccess(e.ctx, e.org, e.owner, r, ip10, time.Time{}, 0)
	require.NoError(t, err)
	assert.Equal(t, Decision{Allow: false, Reasons: []string{"error:compile"}}, d)
}

func TestCheckAccessUnregisteredType(t *testing.T) {
	e := setup(t)
	r := e.resource(t, e.org)
	e.res[e.org][0].Type = "telnet"
	_, err := e.svc.CheckAccess(e.ctx, e.org, e.owner, r, ip10, time.Time{}, 0)
	assert.ErrorIs(t, err, apperr.ErrFailedPrecondition)
}

func TestCheckAccessPort(t *testing.T) {
	e := setup(t)
	r := e.resource(t, e.org)
	e.binds[e.org] = []Binding{{ID: uuid.New(), SubjectType: "user", SubjectID: e.owner, ResourceID: &r,
		Ports: []connector.PortRange{{Port: 5432}}}}
	for port, allow := range map[uint16]bool{5432: true, 5433: false, 0: true} {
		d, err := e.svc.CheckAccess(e.ctx, e.org, e.owner, r, ip10, time.Time{}, port)
		require.NoError(t, err, port)
		assert.Equal(t, allow, d.Allow, port)
	}
}

type fakeGrants []SnapshotGrant

func (f fakeGrants) PolicyGrants(context.Context, uuid.UUID) ([]SnapshotGrant, error) { return f, nil }

type fakeLocks []SnapshotLock

func (f fakeLocks) PolicyLocks(context.Context, uuid.UUID) ([]SnapshotLock, error) { return f, nil }

func TestSnapshotIncludesGrantsAndLocks(t *testing.T) {
	e := setup(t)
	snap, err := e.svc.Snapshot(e.ctx, e.org)
	require.NoError(t, err)
	assert.Empty(t, snap.Grants, "nil sources: none")
	assert.Empty(t, snap.Locks)

	r := e.resource(t, e.org)
	grants := fakeGrants{{ID: uuid.New(), UserID: e.owner, ResourceID: r, ExpiresUnix: time.Now().Add(time.Hour).Unix()}}
	locks := fakeLocks{{ID: uuid.New(), TargetType: "user", TargetID: &e.owner}}
	e.svc.deps.Grants, e.svc.deps.Locks = grants, locks
	snap, err = e.svc.Snapshot(e.ctx, e.org)
	require.NoError(t, err)
	assert.Equal(t, []SnapshotGrant(grants), snap.Grants)
	assert.Equal(t, []SnapshotLock(locks), snap.Locks)

	d, err := e.svc.CheckAccess(e.ctx, e.org, e.owner, r, ip10, time.Time{}, 0)
	require.NoError(t, err)
	assert.Equal(t, []string{"lock:" + locks[0].ID.String()}, d.Reasons, "the grant allows, the lock forbids")
	e.svc.deps.Locks = nil
	d, err = e.svc.CheckAccess(e.ctx, e.org, e.owner, r, ip10, time.Time{}, 0)
	require.NoError(t, err)
	assert.True(t, d.Allow, "the grant allows now")
}

func TestSnapshotIncludesStage11(t *testing.T) {
	e := setup(t)
	prod1, dev, prod2 := e.resource(t, e.org), e.resource(t, e.org), e.resource(t, e.org)
	for i, env := range []string{"prod", "dev", "prod"} {
		e.res[e.org][i].Labels = map[string]string{"env": env}
	}
	byLabel := CommandRuleInput{Name: "prod", Subjects: Subjects{AllUsers: true}, Selector: Selector{Labels: map[string]string{"env": "prod"}},
		Classes: []string{"destructive"}, Effect: "deny"}
	rule, err := e.svc.CreateCommandRule(e.ctx, e.org, byLabel)
	require.NoError(t, err)
	_, err = e.svc.CreateClassRule(e.ctx, e.org, "b*", []string{"write"})
	require.NoError(t, err)
	_, err = e.svc.CreateClassRule(e.ctx, e.org, "a*", []string{"read"})
	require.NoError(t, err)

	snap, err := e.svc.Snapshot(e.ctx, e.org)
	require.NoError(t, err)
	want := []uuid.UUID{prod1, prod2}
	slices.SortFunc(want, compareIDs)
	assert.Equal(t, []SnapshotCommandRule{{ID: rule.ID, Subjects: byLabel.Subjects, ResourceIDs: want, Classes: []string{"destructive"},
		Effect: "deny"}}, snap.CommandRules)
	assert.NotContains(t, snap.CommandRules[0].ResourceIDs, dev)
	assert.Equal(t, []SnapshotClassRule{{Pattern: "a*", Classes: []string{"read"}}, {Pattern: "b*", Classes: []string{"write"}}}, snap.ClassRules)
	assert.Empty(t, snap.CustomPolicies)

	_, text, err := Compile(snap)
	require.NoError(t, err)
	assert.Contains(t, text, `@id("command:`+rule.ID.String()+`")`)
}
