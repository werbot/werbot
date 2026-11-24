//go:build load

package load

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/test/cluster"
)

const (
	relayLine  = 5 * time.Second // every terminal writes one line this often
	relaySteps = time.Minute     // the relay metrics reach the terminal count within this
	relayPath  = "/api/terminal?resource=load-target&cols=80&rows=24"
	dialWait   = 10 * time.Second // one terminal dial
)

// dialTerminal opens a web terminal of alice on api with the Origin of the UI.
func dialTerminal(ctx context.Context, api cluster.API) (*websocket.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, dialWait)
	defer cancel()
	c, _, err := websocket.Dial(ctx, api.BaseURL+relayPath, &websocket.DialOptions{ //nolint:bodyclose // Dial owns the handshake body
		HTTPClient: api.HTTP, HTTPHeader: http.Header{"Origin": {"http://" + cluster.UIHost}},
	})
	return c, err
}

// l5Relays: with gw2 stopped, *relays terminals of server2 reach the host
// target through the only gateway, so every terminal is a browser stream on
// server2 and a bridge on server1 (10 §8.3 L5). They stay *relayHold with one
// line every 5 s. werbot_relay_streams is the count on both replicas, no
// stream is rejected for the limit, RSS is within the §10.1 budgets.
func l5Relays(t *testing.T) {
	_, setOutput := useTarget(t)
	setOutput(0)
	ctx, stop := context.WithCancel(t.Context())
	samples := sampleEvery(ctx, t, st)  // gw2 is resolved here, before it stops
	t.Cleanup(func() { st.Healthy(t) }) // runs after the terminals close
	srv1, srv2 := st.Addr(t, "server1", 9100), st.Addr(t, "server2", 9100)
	_, err := st.Compose(context.Background(), "stop", "gw2")
	require.NoError(t, err)
	require.Eventually(t, func() bool { n, err := cluster.ReadMetric(srv2, "werbot_tunnel_streams"); return err == nil && n == 0 },
		cluster.RestoreMax, time.Second, "server2 holds no tunnel after gw2 stopped")

	var conns []*websocket.Conn
	t.Cleanup(func() {
		for _, c := range conns {
			_ = c.CloseNow()
		}
	})
	for range *relays {
		c, err := dialTerminal(t.Context(), st.B)
		require.NoError(t, err, "terminal %d", len(conns)+1)
		conns = append(conns, c)
	}
	n := int64(*relays)
	streams := func() (browsers, bridges int64) {
		browsers, _ = cluster.ReadMetric(srv2, "werbot_relay_streams")
		bridges, _ = cluster.ReadMetric(srv1, "werbot_relay_streams")
		return browsers, bridges
	}
	require.Eventually(t, func() bool { b, g := streams(); return b == n && g == n }, relaySteps, time.Second, "relay streams on both replicas")

	hold, release := context.WithTimeout(t.Context(), *relayHold)
	defer release()
	broken := holdTerminals(hold, conns)
	<-hold.Done()
	browsers, bridges := streams() // still open after the hold: no terminal was closed by a replica
	release()
	broken.wg.Wait()
	stop()
	all := <-samples

	report(t, "L5", "terminals broken during the hold", "0", fmt.Sprint(broken.errs.Load()), broken.errs.Load() == 0)
	report(t, "L5", "werbot_relay_streams server2 / server1", fmt.Sprintf("%d / %d", n, n), fmt.Sprintf("%d / %d", browsers, bridges), browsers == n && bridges == n)
	rejected := cluster.Metric(t, srv1, "werbot_relay_rejected_total", "reason", "limit") + cluster.Metric(t, srv2, "werbot_relay_rejected_total", "reason", "limit")
	report(t, "L5", "werbot_relay_rejected_total{reason=\"limit\"}", "0", fmt.Sprint(rejected), rejected == 0)
	reportRSS(t, "L5", all, "gw1", gatewayBudget)
	reportRSS(t, "L5", all, "server1", serverBudget)
	reportRSS(t, "L5", all, "server2", serverBudget)
}

// held is the terminals of holdTerminals: errs counts the terminals that failed before the hold ended.
type held struct {
	wg   sync.WaitGroup
	errs atomic.Int64
}

// holdTerminals reads every terminal (a replica pings it and closes it when it
// does not answer) and writes one line to all of them every relayLine until ctx ends.
func holdTerminals(ctx context.Context, conns []*websocket.Conn) *held {
	h := &held{}
	for _, c := range conns {
		h.wg.Go(func() { h.drain(ctx, c) })
	}
	h.wg.Go(func() { h.writeLines(ctx, conns) })
	return h
}

// drain reads c until it fails; a failure before ctx ends is a broken terminal.
func (h *held) drain(ctx context.Context, c *websocket.Conn) {
	for {
		if _, _, err := c.Read(ctx); err != nil {
			if ctx.Err() == nil {
				h.errs.Add(1)
			}
			return
		}
	}
}

func (h *held) writeLines(ctx context.Context, conns []*websocket.Conn) {
	tick := time.NewTicker(relayLine)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		for _, c := range conns {
			if err := c.Write(ctx, websocket.MessageBinary, []byte("echo l5\r")); err != nil && ctx.Err() == nil {
				h.errs.Add(1)
			}
		}
	}
}
