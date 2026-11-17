package gateway

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// denialsAfter waits for the SSH front to finish failed handshakes, then takes the denials.
func (e *sshEnv) denialsAfter(t *testing.T, want int) []*gatewayv1.Denial {
	t.Helper()
	var got []*gatewayv1.Denial
	require.Eventually(t, func() bool {
		got = append(got, e.p.denials.take()...)
		return len(got) >= want
	}, 2*time.Second, 5*time.Millisecond)
	time.Sleep(50 * time.Millisecond) // a second one would show up by now
	return append(got, e.p.denials.take()...)
}

// A refused key reports one auth_failed per connection, however many keys it
// offered; a connection without a key or with an accepted one reports nothing.
func TestAuthFailedDenial(t *testing.T) {
	e := newSSHEnv(t, Config{})
	unknown, other := newKey(t), newKey(t)
	_, err := e.dial("alice+db", unknown, other)
	require.Error(t, err)
	got := e.denialsAfter(t, 1)
	require.Len(t, got, 1)
	d := got[0]
	requireValid(t, &gatewayv1.Heartbeat{Denials: got})
	require.Equal(t, "auth_failed", d.GetReason())
	require.Equal(t, fp(other), d.GetKeyFingerprint(), "the last refused key")
	require.Equal(t, "db", d.GetResource())
	require.Equal(t, "127.0.0.1", d.GetClientIp())
	require.Empty(t, d.GetUserId())

	_, err = e.dial("bob+db", e.keys["alice"]) // alice's key, bob's login
	require.Error(t, err)
	require.Len(t, e.denialsAfter(t, 1), 1)

	c, err := net.Dial("tcp", e.addr) // a port scan
	require.NoError(t, err)
	require.NoError(t, c.Close())
	ok, err := e.dial("alice+db", unknown, e.keys["alice"]) // the second key is accepted
	require.NoError(t, err)
	require.NoError(t, ok.Close())
	time.Sleep(100 * time.Millisecond)
	require.Empty(t, e.p.denials.take())
}

func TestForwardTargetPortZero(t *testing.T) {
	e := newSSHEnv(t, Config{})
	_, err := e.client(t, "alice+tcp").Dial("tcp", "10.0.0.1:0")
	require.ErrorContains(t, err, "Access denied")
	require.Zero(t, e.startCalls())
	require.Equal(t, "target_not_allowed", e.p.denials.take()[0].GetReason())
}
