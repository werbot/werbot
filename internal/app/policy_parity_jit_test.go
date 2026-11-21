package app

import (
	"log/slog"
	"math/rand/v2"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/policy"
)

// TestPolicyParityJIT decides grants and locks on every target the same on
// the server and on the gateway, with context.now and cli_session_id, from one
// database: both decisions must be equal and as expected.
func TestPolicyParityJIT(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	ctx, org, user := s.org(t, "owner")
	p := parity{s: s, org: org, user: user, gw: dbtest.Gateway(t, s.pool, org, "gw"),
		snaps: newTunnelDeps(s.d, metrics.NewRegistry(), slog.New(slog.DiscardHandler)).Snapshots}
	team := dbtest.Team(t, s.pool, org, user)
	f := dbtest.Fixture{OrgID: org}
	ids := map[string]uuid.UUID{}
	for _, name := range []string{"srv", "jit"} {
		ids[name] = dbtest.Resource(t, s.pool, org, name, nil)
		_, err := s.pool.Exec(ctx, "UPDATE resource SET gateway_id = $2, type = 'ssh' WHERE id = $1", ids[name], p.gw)
		require.NoError(t, err)
	}
	p.srv = ids["srv"]
	// srv through a team binding, jit only through the grant
	s.bind(ctx, t, &werbotv1.CreateBindingRequest{SubjectType: "team", SubjectId: team.String(), ResourceId: p.srv.String()})
	grant := f.Grant(t, s.pool, user, ids["jit"], time.Hour)
	cli, other := uuid.New(), uuid.New()
	ip, now := netip.MustParseAddr("10.1.2.3"), time.Now()

	snapshot := func() policy.Snapshot {
		snap, err := s.d.Policy.Snapshot(ctx, org)
		require.NoError(t, err)
		return snap
	}
	g := time.Unix(snapshot().Grants[0].ExpiresUnix, 0)
	// lock adds a lock on target; with ttl it returns its expiry as the snapshot has it.
	lock := func(typ string, target uuid.UUID, ttl time.Duration) time.Time {
		id := f.Lock(t, s.pool, typ, &target)
		if ttl == 0 {
			return time.Time{}
		}
		_, err := s.pool.Exec(ctx, "UPDATE lock SET expires_at = now() + make_interval(secs => $2::float8) WHERE id = $1", id, ttl.Seconds())
		require.NoError(t, err)
		locks := snapshot().Locks
		i := slices.IndexFunc(locks, func(l policy.SnapshotLock) bool { return l.ID == id })
		require.GreaterOrEqual(t, i, 0)
		return time.Unix(locks[i].ExpiresUnix, 0)
	}
	type check struct {
		name    string
		when    time.Time
		session uuid.UUID
		allow   bool
	}
	cases := []struct {
		name   string
		lock   func() time.Time // returns the lock expiry; nil: no lock
		checks func(l time.Time) []check
	}{
		{name: "grant", checks: func(time.Time) []check {
			return []check{{"srv", now, uuid.Nil, true}, {"jit", now, uuid.Nil, true}, {"jit", g.Add(-time.Second), uuid.Nil, true},
				{"jit", g, uuid.Nil, false}, {"jit", g.Add(time.Hour), uuid.Nil, false}, {"srv", g.Add(time.Hour), uuid.Nil, true}}
		}},
		{name: "user lock", lock: func() time.Time { return lock("user", user, 0) }, checks: func(time.Time) []check {
			return []check{{"srv", now, uuid.Nil, false}, {"jit", now, uuid.Nil, false}}
		}},
		{name: "user lock with expiry", lock: func() time.Time { return lock("user", user, 30*time.Minute) }, checks: func(l time.Time) []check {
			return []check{{"srv", now, uuid.Nil, false}, {"srv", l.Add(-time.Second), uuid.Nil, false}, {"srv", l, uuid.Nil, true}, {"jit", l, uuid.Nil, true}}
		}},
		{name: "team lock", lock: func() time.Time { return lock("team", team, 0) }, checks: func(time.Time) []check {
			return []check{{"srv", now, uuid.Nil, false}, {"jit", now, uuid.Nil, false}}
		}},
		{name: "resource lock", lock: func() time.Time { return lock("resource", p.srv, 0) }, checks: func(time.Time) []check {
			return []check{{"srv", now, uuid.Nil, false}, {"jit", now, uuid.Nil, true}}
		}},
		{name: "grant lock", lock: func() time.Time { return lock("grant", grant, 0) }, checks: func(time.Time) []check {
			return []check{{"jit", now, uuid.Nil, false}, {"srv", now, uuid.Nil, true}}
		}},
		{name: "grant lock with expiry", lock: func() time.Time { return lock("grant", grant, 30*time.Minute) }, checks: func(l time.Time) []check {
			return []check{{"jit", now, uuid.Nil, false}, {"jit", l.Add(-time.Second), uuid.Nil, false}, {"jit", l, uuid.Nil, true}, {"jit", g, uuid.Nil, false}}
		}},
		{name: "cli_session lock", lock: func() time.Time { return lock("cli_session", cli, 0) }, checks: func(time.Time) []check {
			return []check{{"srv", now, cli, false}, {"jit", now, cli, false}, {"srv", now, other, true}, {"jit", now, other, true},
				{"srv", now, uuid.Nil, true}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.pool.Exec(ctx, "DELETE FROM lock WHERE org_id = $1", org)
			require.NoError(t, err)
			var l time.Time
			if tc.lock != nil {
				l = tc.lock()
			}
			for _, c := range tc.checks(l) {
				server, gw := p.decide(ctx, t, c.name, ip, "", c.when, 0, c.session)
				assert.Equal(t, c.allow, server, "server: %s at %v, cli %s", c.name, c.when, c.session)
				assert.Equal(t, c.allow, gw, "gateway: %s at %v, cli %s", c.name, c.when, c.session)
			}
		})
	}

	// A cli_session lock and a grant lock with expiry together: 200 random
	// requests (resource, time around both expiries, CLI session) decide alike.
	t.Run("random", func(t *testing.T) {
		_, err := s.pool.Exec(ctx, "DELETE FROM lock WHERE org_id = $1", org)
		require.NoError(t, err)
		lock("cli_session", cli, 0)
		l := lock("grant", grant, 30*time.Minute)
		rng := rand.New(rand.NewPCG(10, 2)) //nolint:gosec // reproducible test requests
		sessions := []uuid.UUID{uuid.Nil, cli, other}
		for range 200 {
			name := []string{"srv", "jit"}[rng.IntN(2)]
			when := l.Add(-time.Minute).Add(time.Duration(rng.Int64N(int64(g.Sub(l) + 2*time.Minute))))
			session := sessions[rng.IntN(len(sessions))]
			want := session != cli && (name == "srv" || (!when.Before(l) && when.Before(g)))
			server, gw := p.decide(ctx, t, name, ip, "", when, 0, session)
			require.Equal(t, want, server, "server: %s at %v, cli %s", name, when, session)
			require.Equal(t, want, gw, "gateway: %s at %v, cli %s", name, when, session)
		}
	})
}
