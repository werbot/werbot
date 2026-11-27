package gateway

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

// pres is a preconn resource; its name is the host Probe dials.
func pres(n int, host string) Resource {
	return Resource{ID: uid(n), Name: host, Type: "preconn", Config: []byte(`{}`)}
}

// A type with a Prober is checked by Probe through the prober's dialer, never by
// the TCP probe; a Probe error is the result's text.
func TestHealthUsesProber(t *testing.T) {
	d := &dialLog{}
	h, _ := healthEnv(d.dial, pres(1, "10.0.0.9"), hres(2, "hprobe", "10.0.0.1"))
	var probed []string
	h.probeDial = func(_ context.Context, network, address string) (net.Conn, error) {
		probed = append(probed, network+" "+address)
		a, b := net.Pipe()
		_ = b.Close()
		return a, nil
	}
	h.pass(t.Context(), uid(0), nil)
	require.Equal(t, []string{"tcp 10.0.0.9:5432"}, probed)
	require.Equal(t, []string{"10.0.0.1:2000", "10.0.0.1:22"}, d.take(), "TCP probes only for the type without a Prober")
	for _, r := range h.take() {
		require.True(t, r.GetOk(), r.GetResourceId())
	}

	h.probeDial = func(context.Context, string, string) (net.Conn, error) { return nil, errors.New("probe refused") }
	got := h.check(t.Context(), probeTarget{id: uid(1), host: "10.0.0.9", ports: []uint16{5432}, res: pres(1, "10.0.0.9"), probe: preConn{}})
	require.False(t, got.ok)
	require.Equal(t, "probe refused", got.err)
	require.Empty(t, d.take())
}

// The pipeline's prober hands Probe the guarded dialer: a refused local target
// fails the check without a connection (Rulings C9, G13).
func TestHealthProberUsesGuardedDial(t *testing.T) {
	f := &fakeControl{connected: true}
	p := newPipeline(f, Config{DenyLocalTargets: true}, slog.New(slog.DiscardHandler))
	t.Cleanup(p.stopNow)
	got := p.health.check(t.Context(), probeTarget{id: uid(1), host: "127.0.0.1", ports: []uint16{5432}, res: pres(1, "127.0.0.1"), probe: preConn{}})
	require.False(t, got.ok)
	require.Contains(t, got.err, ErrTargetNotAllowed.Error())
}
