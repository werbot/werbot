package policy

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

func TestRPCCheckAccess(t *testing.T) {
	e := setup(t)
	h := NewHandler(e.svc)
	r := e.resource(t, e.org)
	e.bind(e.org, e.owner, r)
	req := func(ip string) *connect.Request[werbotv1.CheckAccessRequest] {
		return connect.NewRequest(&werbotv1.CheckAccessRequest{UserId: e.owner.String(), ResourceId: r.String(), Ip: ip})
	}

	_, err := h.CheckAccess(e.ctx, req("bad"))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	tooHigh := req("10.1.2.3")
	tooHigh.Msg.Port = 65536
	_, err = h.CheckAccess(e.ctx, tooHigh)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "port above 65535")

	member := dbtest.User(t, e.pool, e.org, "member")
	mctx := principal.With(context.Background(), principal.Principal{UserID: member, OrgID: e.org, Role: "member"})
	_, err = h.CheckAccess(mctx, req("10.1.2.3"))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = h.GetResourceRules(mctx, connect.NewRequest(&werbotv1.GetResourceRulesRequest{ResourceId: r.String()}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = h.SetResourceRules(mctx, connect.NewRequest(&werbotv1.SetResourceRulesRequest{ResourceId: r.String()}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))

	_, err = e.pool.Exec(context.Background(), "INSERT INTO ip_blocklist (cidr, source) VALUES ('203.0.113.0/24', 'test')")
	require.NoError(t, err)
	for _, ip := range []string{"10.1.2.3", "203.0.113.7"} {
		res, err := h.CheckAccess(e.ctx, req(ip))
		require.NoError(t, err, ip)
		want, err := e.svc.CheckAccess(e.ctx, e.org, e.owner, r, netip.MustParseAddr(ip), time.Time{}, 0)
		require.NoError(t, err, ip)
		assert.Equal(t, ip == "10.1.2.3", res.Msg.GetAllow(), ip)
		assert.Equal(t, want.Allow, res.Msg.GetAllow(), ip)
		assert.Equal(t, want.Reasons, res.Msg.GetReasons(), ip)
	}

	e.binds[e.org][0].Ports = []connector.PortRange{{Port: 5432}}
	for port, allow := range map[uint32]bool{5432: true, 5433: false} {
		pr := req("10.1.2.3")
		pr.Msg.Port = port
		res, err := h.CheckAccess(e.ctx, pr)
		require.NoError(t, err, port)
		assert.Equal(t, allow, res.Msg.GetAllow(), port)
	}
}

func TestRPCResourceRulesRoundTrip(t *testing.T) {
	e := setup(t)
	h := NewHandler(e.svc)
	r := e.resource(t, e.org)

	_, err := h.SetResourceRules(e.ctx, connect.NewRequest(&werbotv1.SetResourceRulesRequest{ResourceId: r.String(),
		Networks: []*werbotv1.NetworkRule{{Cidr: "x", Effect: "allow"}}}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	_, err = h.SetResourceRules(e.ctx, connect.NewRequest(&werbotv1.SetResourceRulesRequest{ResourceId: r.String(),
		Networks:  []*werbotv1.NetworkRule{{Cidr: "10.0.0.1/8", Effect: "deny"}},
		Countries: []*werbotv1.CountryRule{{Country: "FR", Effect: "allow"}},
		Schedule:  []*werbotv1.ScheduleSlot{{Weekday: 2, Hour: 23}},
	}))
	require.NoError(t, err)
	got, err := h.GetResourceRules(e.ctx, connect.NewRequest(&werbotv1.GetResourceRulesRequest{ResourceId: r.String()}))
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.0/8", got.Msg.GetNetworks()[0].GetCidr())
	assert.Equal(t, "deny", got.Msg.GetNetworks()[0].GetEffect())
	assert.Equal(t, "FR", got.Msg.GetCountries()[0].GetCountry())
	assert.Equal(t, int32(23), got.Msg.GetSchedule()[0].GetHour())
}
