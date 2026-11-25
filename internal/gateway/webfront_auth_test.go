package gateway

import (
	"context"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// tunnelAuthControl is a fake control plane that authenticates users through
// a tunnel client.
type tunnelAuthControl struct {
	*fakeControl
	c *tunnelClient
}

func (a tunnelAuthControl) AuthenticateUser(ctx context.Context, id Identity, ip netip.Addr) (User, error) {
	return a.c.AuthenticateUser(ctx, id, ip)
}

func TestWebTicketObserveModeDenied(t *testing.T) {
	e := newPipeEnv(t, 10)
	f := &webTunnel{answer: &gatewayv1.AuthenticateUserResponse{UserId: e.f.user.ID.String(), Login: "alice",
		Web: &gatewayv1.WebTicket{Mode: "observe", SessionId: uuid.NewString(), ClientIp: browserIP.String()}}}
	e.p.ctl = tunnelAuthControl{fakeControl: e.f, c: newWebTunnel(t, f)}
	ticket := uuid.New()
	stale := &Handshake{User: e.f.user, At: time.Now().Add(-time.Minute)} // the ticket is looked up again
	r := OpenRequest{Front: "web", Kind: KindTerminal, Identity: Identity{Method: "web-ticket", Value: ticket.String()},
		Handshake: stale, Resource: "fake", ClientIP: browserIP, Window: Window{120, 40}}

	_, err := e.p.Open(t.Context(), r)
	require.ErrorIs(t, err, ErrDenied, "an observe ticket never opens a terminal")
	require.Zero(t, e.startCalls())
	f.mu.Lock()
	require.Len(t, f.auths, 1)
	assert.Equal(t, ticket.String(), f.auths[0].GetWebTicket(), "a web identity is looked up by its ticket")
	assert.Equal(t, browserIP.String(), f.auths[0].GetClientIp())
	f.answer.Web.Mode = "web"
	f.mu.Unlock()

	s, err := e.p.Open(t.Context(), r)
	require.NoError(t, err, "a web ticket opens it")
	e.p.abort(s)
}

func TestServeWiresWebFront(t *testing.T) {
	e := newServeEnv(t, time.Second)
	pushMsg(t, e.tun, &gatewayv1.ConnectResponse{Msg: &gatewayv1.ConnectResponse_OpenStream{OpenStream: &gatewayv1.OpenStream{TicketId: uuid.NewString()}}})
	require.Eventually(t, func() bool {
		var ips []string
		e.tun.with(func(f *fakeTunnel) { ips = f.unaryIP })
		return slices.Contains(ips, "0.0.0.0")
	}, 5*time.Second, 10*time.Millisecond, "an open_stream reaches the web front, which looks its ticket up")
}
