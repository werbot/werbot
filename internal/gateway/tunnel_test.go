package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

func TestTunnelReceivesSnapshotAndHeartbeats(t *testing.T) {
	e := newTunnelEnv(t, nil, time.Hour)
	require.Contains(t, e.metrics(t), "werbot_gateway_tunnel_connected 0")
	require.Nil(t, e.c.Policy())
	sid := uuid.NewString()
	e.run(t, func() *gatewayv1.Heartbeat {
		return &gatewayv1.Heartbeat{Version: "1.2.3", Connectors: []string{"ssh"}, ActiveSessionIds: []string{sid}}
	})

	require.Eventually(t, func() bool { return e.c.Policy() != nil && e.c.Connected() }, 2*time.Second, 5*time.Millisecond)
	require.Equal(t, "v1", e.c.Policy().Version())
	require.Contains(t, e.metrics(t), "werbot_gateway_tunnel_connected 1")
	require.Eventually(t, func() bool { // the first heartbeat and at least one tick
		e.f.mu.Lock()
		defer e.f.mu.Unlock()
		return len(e.f.hbs) >= 2
	}, 2*time.Second, 5*time.Millisecond)
	e.f.mu.Lock()
	hb := e.f.hbs[1]
	e.f.mu.Unlock()
	require.Equal(t, "1.2.3", hb.GetVersion())
	require.Equal(t, []string{sid}, hb.GetActiveSessionIds())
	require.Equal(t, []string{"ssh"}, hb.GetConnectors())
}

func TestTunnelReconnectKeepsSnapshot(t *testing.T) {
	e := newTunnelEnv(t, func(n int) attempt {
		switch n {
		case 0:
			return attempt{snap: testSnap("v1"), closeWith: connect.NewError(connect.CodeInternal, errors.New("boom"))}
		case 1, 2:
			return attempt{reject: connect.NewError(connect.CodeInternal, errors.New("no"))}
		}
		return attempt{snap: testSnap("v1")}
	}, time.Hour)
	e.run(t, nil)

	require.Eventually(t, func() bool { return e.f.attempts() >= 3 }, 2*time.Second, time.Millisecond)
	p := e.c.Policy()
	require.NotNil(t, p)
	require.False(t, e.c.Connected())
	require.Contains(t, e.metrics(t), "werbot_gateway_tunnel_connected 0")
	require.Eventually(t, func() bool { return e.f.attempts() == 4 && e.c.Connected() }, 2*time.Second, time.Millisecond)
	require.Same(t, p, e.c.Policy())

	e.f.mu.Lock()
	defer e.f.mu.Unlock()
	for i := range 3 {
		require.GreaterOrEqual(t, e.f.starts[i+1].Sub(e.f.closes[i]), e.c.backoffMin, "pause before attempt %d", i+1)
	}
}

func TestTunnelBackoffResetsAfterSnapshot(t *testing.T) {
	fail := connect.NewError(connect.CodeInternal, errors.New("no"))
	e := newTunnelEnv(t, func(n int) attempt {
		switch n {
		case 0, 1:
			return attempt{reject: fail}
		case 2:
			return attempt{snap: testSnap("v1"), closeWith: fail}
		}
		return attempt{snap: testSnap("v1")}
	}, time.Hour)
	e.c.backoffMin = 200 * time.Millisecond
	e.run(t, nil)
	require.Eventually(t, func() bool { return e.f.attempts() == 4 }, 3*time.Second, time.Millisecond)

	e.f.mu.Lock()
	defer e.f.mu.Unlock()
	require.Less(t, e.f.starts[3].Sub(e.f.closes[2]), 500*time.Millisecond, "a snapshot resets the backoff (else ≥ 640ms)")
}

func TestTunnelRebalanceReconnectsImmediately(t *testing.T) {
	codes := []string{"rebalance", "shutdown", "heartbeat timeout"}
	e := newTunnelEnv(t, func(n int) attempt {
		a := attempt{snap: testSnap("v1")}
		if n < len(codes) {
			a.closeWith = connect.NewError(connect.CodeUnavailable, errors.New(codes[n]))
		}
		return a
	}, time.Hour)
	e.c.backoffMin = 500 * time.Millisecond
	e.run(t, nil)

	require.Eventually(t, func() bool { return e.c.Policy() != nil }, 2*time.Second, time.Millisecond)
	p := e.c.Policy()
	require.Eventually(t, func() bool { return e.f.attempts() == 4 && e.c.Connected() }, 3*time.Second, time.Millisecond)
	require.Same(t, p, e.c.Policy())

	e.f.mu.Lock()
	defer e.f.mu.Unlock()
	require.Less(t, e.f.starts[1].Sub(e.f.closes[0]), 100*time.Millisecond, "rebalance")
	require.Less(t, e.f.starts[2].Sub(e.f.closes[1]), 100*time.Millisecond, "shutdown")
	require.GreaterOrEqual(t, e.f.starts[3].Sub(e.f.closes[2]), 500*time.Millisecond, "heartbeat timeout")
}

// A planned close (rebalance, shutdown) keeps the gateway ready while it reconnects,
// for at most reconnectGrace; a failed reconnect or an unplanned break ends it at once.
func TestTunnelPlannedCloseKeepsReady(t *testing.T) {
	planned := connect.NewError(connect.CodeUnavailable, errors.New("rebalance"))
	env := func(t *testing.T, first error, second attempt, grace time.Duration) *tunnelEnv {
		e := newTunnelEnv(t, func(n int) attempt {
			switch n {
			case 0:
				return attempt{snap: testSnap("v1"), closeWith: first}
			case 1:
				return second
			}
			return attempt{snap: testSnap("v1")}
		}, time.Hour)
		e.c.reconnectGrace = grace
		e.run(t, nil)
		require.Eventually(t, func() bool { return e.f.attempts() == 2 }, 2*time.Second, time.Millisecond)
		return e
	}
	t.Run("ready until the snapshot", func(t *testing.T) {
		hold := make(chan struct{})
		e := env(t, planned, attempt{snap: testSnap("v1"), hold: hold}, time.Hour)
		require.Never(t, func() bool { return !e.c.Connected() }, 200*time.Millisecond, time.Millisecond)
		close(hold)
		require.Eventually(t, func() bool { return e.c.Connected() && e.f.attempts() == 2 }, 2*time.Second, time.Millisecond)
	})
	t.Run("grace expires", func(t *testing.T) {
		e := env(t, planned, attempt{snap: testSnap("v1"), hold: make(chan struct{})}, 100*time.Millisecond)
		require.Eventually(t, func() bool { return !e.c.Connected() }, 2*time.Second, time.Millisecond)
	})
	t.Run("failed reconnect", func(t *testing.T) {
		e := env(t, planned, attempt{reject: connect.NewError(connect.CodeInternal, errors.New("no"))}, time.Hour)
		require.Eventually(t, func() bool { return !e.c.Connected() }, 2*time.Second, time.Millisecond)
	})
	t.Run("unplanned break", func(t *testing.T) {
		e := env(t, connect.NewError(connect.CodeInternal, errors.New("boom")), attempt{snap: testSnap("v1"), hold: make(chan struct{})}, time.Hour)
		require.Eventually(t, func() bool { return !e.c.Connected() }, 2*time.Second, time.Millisecond)
	})
}

func TestTunnelSkipsSameVersion(t *testing.T) {
	e := newTunnelEnv(t, nil, time.Hour)
	e.run(t, nil)
	require.Eventually(t, func() bool { return e.c.Connected() }, 2*time.Second, time.Millisecond)
	p := e.c.Policy()

	e.f.push <- &gatewayv1.Snapshot{Version: "v1", CedarPolicies: "garbage ((("}
	e.f.push <- testSnap("v2")
	require.Eventually(t, func() bool { return e.c.Policy() != p }, 2*time.Second, time.Millisecond)
	require.Equal(t, "v2", e.c.Policy().Version())
	require.NotContains(t, e.log.String(), "compile")
}

func TestTunnelBrokenSnapshotNotReady(t *testing.T) {
	e := newTunnelEnv(t, nil, time.Hour)
	e.run(t, nil)
	require.Eventually(t, e.c.Connected, 2*time.Second, time.Millisecond)
	p := e.c.Policy()

	// a revoke the gateway cannot compile: keep v1 for active sessions, refuse new ones
	e.f.push <- &gatewayv1.Snapshot{Version: "v3", CedarPolicies: "garbage ((("}
	require.Eventually(t, func() bool { return !e.c.Connected() }, 2*time.Second, time.Millisecond)
	require.Contains(t, e.log.String(), "compile")
	require.Contains(t, e.metrics(t), "werbot_gateway_tunnel_connected 0")
	require.Same(t, p, e.c.Policy())
	require.Equal(t, "v1", e.c.Policy().Version())

	e.f.push <- testSnap("v4")
	require.Eventually(t, e.c.Connected, 2*time.Second, time.Millisecond)
	require.Equal(t, "v4", e.c.Policy().Version())
	require.Contains(t, e.metrics(t), "werbot_gateway_tunnel_connected 1")
}

func TestTunnelShutdownRejectBacksOff(t *testing.T) {
	// a replica after Hub.Shutdown rejects every new stream before any snapshot
	e := newTunnelEnv(t, func(int) attempt {
		return attempt{reject: connect.NewError(connect.CodeUnavailable, errors.New("shutdown"))}
	}, time.Hour)
	e.c.backoffMin = 50 * time.Millisecond
	e.run(t, nil)
	require.Eventually(t, func() bool { return e.f.attempts() >= 4 }, 3*time.Second, time.Millisecond)

	e.f.mu.Lock()
	defer e.f.mu.Unlock()
	for i := range 3 {
		require.GreaterOrEqual(t, e.f.starts[i+1].Sub(e.f.closes[i]), e.c.backoffMin, "pause before attempt %d", i+1)
	}
}

func TestTunnelBackoff(t *testing.T) {
	c := &tunnelClient{backoffMin: time.Second, backoffMax: 30 * time.Second}
	for n, want := range []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 30 * time.Second} {
		if n == 3 {
			n = 100 // capped, no overflow
		}
		for range 50 {
			d := backoff(n, c.backoffMin, c.backoffMax)
			require.GreaterOrEqual(t, d, max(want*8/10, c.backoffMin))
			require.LessOrEqual(t, d, want*12/10)
		}
	}
}

// A heartbeat that hits a stream the server already closed gets io.EOF; the server's
// close error from Receive must still decide the reconnect: a planned close never backs off.
func TestHeartbeatEOFKeepsServerError(t *testing.T) {
	const n = 30
	e := newTunnelEnv(t, func(int) attempt {
		return attempt{snap: testSnap("v1"), closeWith: connect.NewError(connect.CodeUnavailable, errors.New("rebalance")),
			closeAfter: 20 * time.Millisecond}
	}, time.Hour)
	e.c.heartbeatEvery, e.c.backoffMin = time.Millisecond, 500*time.Millisecond
	e.run(t, nil)
	require.Eventually(t, func() bool { return e.f.attempts() > n }, 10*time.Second, time.Millisecond)

	e.f.mu.Lock()
	defer e.f.mu.Unlock()
	for i := range n {
		require.Less(t, e.f.starts[i+1].Sub(e.f.closes[i]), e.c.backoffMin, "reconnect %d backed off", i+1)
	}
}

// Once the client certificate expired no stream can open and renewal is impossible:
// Run returns errCertExpired so the process restarts and enrolls again.
func TestTunnelStopsOnExpiredCert(t *testing.T) {
	e := newTunnelEnv(t, func(int) attempt {
		return attempt{reject: connect.NewError(connect.CodeInternal, errors.New("no"))}
	}, time.Second)
	e.f.with(func(f *fakeTunnel) { f.renewErr = connect.NewError(connect.CodeUnavailable, errors.New("down")) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- e.c.Run(ctx, func() *gatewayv1.Heartbeat { return &gatewayv1.Heartbeat{Version: "test"} })
	}()
	select {
	case err := <-done:
		require.ErrorIs(t, err, errCertExpired)
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after the certificate expired")
	}
}
