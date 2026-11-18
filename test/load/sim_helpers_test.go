//go:build load

package load

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/gen/gateway/v1/gatewayv1connect"
)

// Pure helpers of L2; they run on any OS: go test -tags load -short ./test/load/

func TestSimNext(t *testing.T) {
	t0 := time.Now()
	s := &sim{name: "sim-0", got: make(chan snap, 4)}
	s.got <- snap{version: "a", at: t0.Add(time.Second)}
	s.got <- snap{version: "b", at: t0.Add(2 * time.Second), size: 7}
	got, err := s.next("a", t0.Add(versionWait))
	require.NoError(t, err)
	assert.Equal(t, snap{version: "b", at: t0.Add(2 * time.Second), size: 7}, got, "the same version is skipped")

	_, err = s.next("b", time.Now().Add(10*time.Millisecond))
	assert.ErrorContains(t, err, "no new version", "nothing arrives before the deadline")

	s.got <- snap{version: "c", at: t0.Add(versionWait + time.Millisecond)}
	_, err = s.next("b", t0.Add(versionWait))
	assert.ErrorContains(t, err, "after the deadline", "a late snapshot does not count")

	s.err = errors.New("heartbeat timeout")
	close(s.got)
	_, err = s.next("b", time.Now().Add(time.Second))
	assert.ErrorContains(t, err, "heartbeat timeout", "a closed stream returns at once with its error")
}

func TestDrain(t *testing.T) {
	ch := make(chan snap, 3)
	assert.Equal(t, "a", drain(ch, "a"), "nothing buffered")
	ch <- snap{version: "b"}
	ch <- snap{version: "c"}
	assert.Equal(t, "c", drain(ch, "a"))
	close(ch)
	assert.Equal(t, "c", drain(ch, "c"), "a closed channel keeps the version")
}

func TestVersionRows(t *testing.T) {
	lat := make([]time.Duration, 0, 200)
	for i := 1; i <= 200; i++ {
		lat = append(lat, time.Duration(i)*20*time.Millisecond) // 20 ms .. 4 s
	}
	got, pass := versionRow(lat, 200)
	assert.True(t, pass)
	assert.Equal(t, "3.96s", got, "nearest rank: 198th of 200")

	got, pass = versionRow(append(lat[:199:199], 6*time.Second), 200)
	assert.True(t, pass, "one slow sim of 200 is above p99")
	assert.Equal(t, "3.96s", got)
	_, pass = versionRow(append(lat[:197:197], 6*time.Second, 6*time.Second, 6*time.Second), 200)
	assert.False(t, pass, "three slow sims of 200 reach p99")

	got, pass = versionRow(lat[:150], 200)
	assert.False(t, pass, "50 sims never saw the version")
	assert.Equal(t, "2.98s (150 of 200)", got)
	got, pass = versionRow(nil, 200)
	assert.False(t, pass, "no sim connected")
	assert.Equal(t, "0s (0 of 200)", got)
	_, pass = versionRow(nil, 0)
	assert.False(t, pass, "nothing measured")

	got, pass = receivedRow(200, 200)
	assert.True(t, pass)
	assert.Equal(t, "200 of 200", got)
	_, pass = receivedRow(199, 200)
	assert.False(t, pass)
	_, pass = receivedRow(0, 0)
	assert.False(t, pass, "nothing measured")
}

func TestRSSRow(t *testing.T) {
	samples := []sample{
		{RSS: map[string]int64{"server1": 310_000_000}, Active: map[string]int64{"server1": 202}},
		{RSS: map[string]int64{}},
	}
	got, n, over := rssRow(samples, "server1", serverBudget)
	assert.Equal(t, "max 310.0 MB, budget 312.9 MB at 202 active; 0 of 1 samples over", got)
	assert.Equal(t, [2]int{1, 0}, [2]int{n, over})
	_, n, _ = rssRow(samples, "server2", serverBudget)
	assert.Zero(t, n, "server2 never measured")
}

// tunnelFake sends two snapshots after the first heartbeat, then ends the stream.
type tunnelFake struct {
	gatewayv1connect.UnimplementedTunnelServiceHandler
	beat chan *gatewayv1.Heartbeat
}

func (f *tunnelFake) Connect(_ context.Context, st *connect.BidiStream[gatewayv1.ConnectRequest, gatewayv1.ConnectResponse]) error {
	msg, err := st.Receive()
	if err != nil {
		return err
	}
	f.beat <- msg.GetHeartbeat()
	for _, v := range []string{"v1", "v2"} {
		snapshot := &gatewayv1.Snapshot{Version: v, CedarPolicies: "permit (principal, action, resource);"}
		if err := st.Send(&gatewayv1.ConnectResponse{Msg: &gatewayv1.ConnectResponse_Snapshot{Snapshot: snapshot}}); err != nil {
			return err
		}
	}
	return connect.NewError(connect.CodeUnavailable, errors.New("rebalance"))
}

// TestSimRun runs a sim against a fake tunnel over HTTP/2: heartbeat first,
// every snapshot reaches got with its size, a stream end closes got.
func TestSimRun(t *testing.T) {
	fake := &tunnelFake{beat: make(chan *gatewayv1.Heartbeat, 1)}
	mux := http.NewServeMux()
	mux.Handle(gatewayv1connect.NewTunnelServiceHandler(fake))
	srv := httptest.NewUnstartedServer(mux)
	srv.EnableHTTP2 = true
	srv.StartTLS()
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second) // a broken run fails instead of hanging
	defer cancel()
	s := &sim{name: "sim-0", got: make(chan snap, 4)}
	go s.run(ctx, gatewayv1connect.NewTunnelServiceClient(srv.Client(), srv.URL))
	var hb *gatewayv1.Heartbeat
	select {
	case hb = <-fake.beat:
	case <-ctx.Done():
		require.FailNow(t, "no heartbeat")
	}
	assert.Equal(t, "load", hb.GetVersion())
	assert.Equal(t, []string{"ssh"}, hb.GetConnectors())
	var versions []string
	for x := range s.got {
		assert.Positive(t, x.size)
		assert.False(t, x.at.Before(s.start), "stamped after the stream opened")
		versions = append(versions, x.version)
	}
	assert.Equal(t, []string{"v1", "v2"}, versions)
	assert.ErrorContains(t, s.err, "rebalance")
}
