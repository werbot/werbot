package app

import (
	"fmt"
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

// TestPolicyParityMFA decides connect requests with every factor age and
// method under MFA rules of each strength on the server (policy.PolicySet)
// and on the gateway (CompilePolicy of the snapshot the tunnel builds) from
// one database: Allow, Reasons, Obligations, Message and the required
// strength must be equal; an agent of the user is denied plainly on both.
func TestPolicyParityMFA(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	ctx, org, user := s.org(t, "owner")
	p := agentParity{parity: parity{s: s, org: org, user: user, gw: dbtest.Gateway(t, s.pool, org, "gw"),
		snaps: newTunnelDeps(s.d, metrics.NewRegistry(), slog.New(slog.DiscardHandler)).Snapshots}, agent: uuid.New(), run: uuid.New()}
	team := dbtest.Team(t, s.pool, org, user)
	srv, err := s.d.Inventory.Create(ctx, org, inventory.CreateInput{Name: "srv", Type: "ssh", GatewayID: &p.gw,
		Config: &sshv1.Config{Host: "db.internal", Login: "root"}})
	require.NoError(t, err)
	p.srv = srv.ID
	s.bind(ctx, t, &werbotv1.CreateBindingRequest{SubjectType: "team", SubjectId: team.String(), ResourceId: p.srv.String()})
	rule := func(strength string, maxAge time.Duration) {
		_, err := s.d.Policy.CreateMFARule(ctx, org, policy.MFARuleInput{Name: strength + uuid.NewString()[:8], Subjects: policy.Subjects{Teams: []uuid.UUID{team}},
			Selector: policy.Selector{ResourceIDs: []uuid.UUID{p.srv}}, MaxAge: maxAge, Strength: strength})
		require.NoError(t, err)
	}
	ages := []int64{0, 59, 61, policy.NoMFA}
	methods := []string{"totp", "webauthn", ""}
	for _, rules := range [][]string{nil, {"any"}, {"webauthn"}, {"any", "webauthn"}} {
		t.Run(fmt.Sprint(rules), func(t *testing.T) {
			_, err := s.pool.Exec(ctx, "DELETE FROM mfa_rule WHERE org_id = $1", org)
			require.NoError(t, err)
			for _, strength := range rules {
				rule(strength, time.Minute)
			}
			ps, pol, teams := p.compiledWith(ctx, t, nil)
			r, ok := pol.Resource("srv")
			require.True(t, ok)
			for _, age := range ages {
				for _, method := range methods {
					name := fmt.Sprint(age, "/", method)
					sreq := policy.Request{UserID: user, TeamIDs: teams, ResourceID: p.srv, Action: "ssh.connect", IP: netip.MustParseAddr("10.1.2.3"),
						Time: monday10, MFAAgeSeconds: age, MFAMethod: method}
					greq := gateway.AccessRequest{User: gateway.User{ID: user, TeamIDs: teams}, Resource: r, IP: sreq.IP, Time: monday10,
						MFAAgeSeconds: age, MFAMethod: method}
					server, gw := ps.Authorize(sreq), pol.Decide(greq)
					requireSame(t, server, gw, name)
					assert.Equal(t, policy.MFAStrength(ps, sreq), pol.MFAStrength(greq), name)
					assert.Equal(t, len(rules) == 0 || (age < 60 && method != "" && (method == "webauthn" || len(rules) == 1 && rules[0] == "any")),
						server.Allow, name)

					sreq.AgentID, sreq.RunID, sreq.Kind = &p.agent, &p.run, "exec"
					greq.User.Agent, greq.Kind = &gateway.Agent{ID: p.agent, RunID: p.run, AuthAt: monday10.Add(-time.Hour)}, gateway.KindExec
					server, gw = ps.Authorize(sreq), pol.Decide(greq)
					requireSame(t, server, gw, name+" agent")
					assert.Nil(t, server.Obligations, name+" agent")
				}
			}
		})
	}
}
