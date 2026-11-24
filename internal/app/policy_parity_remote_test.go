package app

import (
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sshv1 "github.com/werbot/werbot/gen/werbot/connector/ssh/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/policy"
)

// TestBindingPermitsRemoteForward decides ssh.remote_forward on the server
// (policy.PolicySet) and on the gateway (CompilePolicy of the snapshot the
// tunnel builds) from one database: a binding permits it to a person, an
// agent's pass denies it, a custom forbid on the action denies it; both
// sides agree on Allow, Reasons, Obligations and Message.
func TestBindingPermitsRemoteForward(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	ctx, org, user := s.org(t, "owner")
	p := agentParity{parity: parity{s: s, org: org, user: user, gw: dbtest.Gateway(t, s.pool, org, "gw"),
		snaps: newTunnelDeps(s.d, metrics.NewRegistry(), slog.New(slog.DiscardHandler)).Snapshots}, agent: uuid.New(), run: uuid.New()}
	team := dbtest.Team(t, s.pool, org, user)
	srv, err := s.d.Inventory.Create(ctx, org, inventory.CreateInput{Name: "srv", Type: "ssh", GatewayID: &p.gw,
		Config: &sshv1.Config{Host: "db.internal", Login: "root", RemoteForwarding: true}})
	require.NoError(t, err)
	p.srv = srv.ID
	binding := s.bind(ctx, t, &werbotv1.CreateBindingRequest{SubjectType: "team", SubjectId: team.String(), ResourceId: p.srv.String()})
	custom := policy.SnapshotCustomPolicy{ID: uuid.New(), Cedar: `forbid (principal, action == Werbot::Action::"ssh.remote_forward", resource);`}
	cases := []struct {
		name   string
		agent  bool
		edit   func(*policy.Snapshot)
		allow  bool
		reason string
	}{
		{name: "binding", allow: true, reason: "binding:" + binding},
		{name: "agent", agent: true},
		{name: "custom forbid", reason: "custom:" + custom.ID.String() + ":0",
			edit: func(s *policy.Snapshot) { s.CustomPolicies = append(s.CustomPolicies, custom) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ps, pol, teams := p.compiledWith(ctx, t, tc.edit)
			r, ok := pol.Resource("srv")
			require.True(t, ok)
			ip := netip.MustParseAddr("10.1.2.3")
			req := gateway.AccessRequest{User: gateway.User{ID: user, TeamIDs: teams}, Resource: r, IP: ip, Time: monday10,
				Action: "ssh.remote_forward", Kind: gateway.KindStream, Port: 8080}
			sreq := policy.Request{UserID: user, TeamIDs: teams, ResourceID: r.ID, Action: "ssh.remote_forward", IP: ip, Time: monday10,
				Kind: string(gateway.KindStream), Port: 8080}
			if tc.agent {
				req.User.Agent = &gateway.Agent{ID: p.agent, RunID: p.run, AuthAt: monday10.Add(-time.Hour)}
				sreq.AgentID, sreq.RunID = &p.agent, &p.run
			}
			server, gw := ps.Authorize(sreq), pol.Decide(req)
			requireSame(t, server, gw, tc.name)
			assert.Equal(t, tc.allow, server.Allow, "%v", server.Reasons)
			if tc.reason != "" {
				assert.Contains(t, server.Reasons, tc.reason)
			}
		})
	}
}
