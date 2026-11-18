//go:build load

package load

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/test/cluster"
)

const (
	sampleInterval = 5 * time.Second
	mixSteadyExtra = 30 * time.Second // beyond n_mix/rate: the mix reaches steady state within this
)

// TestL1: *sessions sessions through the pool (nginx :2222), 1 KB/s each for
// *duration (10 §8.3), beside the stage 6b mix (steady before the ramp starts)
// through one werbot connect:
// *tcpConns TCP connections at 1 KB/s and *udpBinds UDP bindings at one
// datagram a second (11 §9). No errors, connect p99 ≤ 1 s, spools drained and
// every session ended within 5 min, every recording verified, RSS (TCP and UDP
// sessions included) and Postgres connections within the §10.1 budgets in every sample.
func TestL1(t *testing.T) {
	needStand(t)
	resource, setOutput := useTarget(t)
	setOutput(1024)
	waitTarget(t, nginxAddr(t, 2201), nginxAddr(t, 2202))
	pool := nginxAddr(t, 2222)
	tcp, udp := startMix(t)

	from := time.Now()
	ctx, stop := context.WithCancel(t.Context())
	samples := sampleEvery(ctx, t, st, sampleInterval)
	mixCtx, stopMix := context.WithCancel(t.Context())
	mixed := runMix(mixCtx, tcp, udp, *rate)
	if n := *tcpConns + len(udp); n > 0 { // steady state first: the measured ramp must not overlap the mix ramp
		limit := time.Duration(n)*time.Second/time.Duration(max(*rate, 1)) + mixSteadyExtra
		took, ok := mixed.waitSteady(t.Context(), limit)
		got, pass := steadyRow(took, limit, ok, mixed.Dead[0].Load(), mixed.Dead[1].Load())
		report(t, "L1", "mix ramp (TCP and UDP echoed once)", "reference", got, pass)
		if !pass {
			stopMix()
			stop()
			reportMix(t, <-mixed.Results)
			return
		}
	}
	res := runSessions(t.Context(), t, st, []string{pool}, *sessions, *rate, *duration, true)
	stopMix()
	mix := <-mixed.Results
	s := settle(t, st, resource, from, *sessions) // sampled too: spool upload and ingest (P7-P9)
	stop()
	all := <-samples

	errs := failed(t, res)
	report(t, "L1", "errors", "0", fmt.Sprint(errs), errs == 0)
	reportMix(t, mix)
	p := p99(connects(res))
	report(t, "L1", "connect p99", "≤ 1s", p.Round(time.Millisecond).String(), p > 0 && p <= time.Second)
	if !s.OK {
		t.Logf("not settled: %s", s)
	}
	report(t, "L1", "spool drained, sessions ended, recordings finished", fmt.Sprintf("≤ %s, %d each", drainMax, *sessions),
		fmt.Sprintf("%s, %d ended, %d finished", s.Took.Round(time.Second), s.Ended, s.Finished), s.OK)
	v := verifyAll(t, st, s.Sessions)
	report(t, "L1", "recordings verified", fmt.Sprintf("all %d", *sessions), fmt.Sprint(v), v == *sessions)
	reportRSS(t, "L1", all, "gw1", gatewayBudget)
	reportRSS(t, "L1", all, "gw2", gatewayBudget)
	reportRSS(t, "L1", all, "server1", serverBudget)
	reportRSS(t, "L1", all, "server2", serverBudget)
	reportPG(t, "L1", all)
	reportSlow(t, "L1", st)
}

// TestL3 is L1 with server1 killed at *duration/2: no session breaks (10 §8.3).
func TestL3(t *testing.T) {
	needStand(t)
	// the kill must land inside every session's hold: the ramp ends before *duration/2
	ramp := time.Duration(*sessions) * time.Second / time.Duration(max(*rate, 1))
	require.Less(t, ramp, *duration/2, "ramp sessions/rate must be shorter than duration/2")
	resource, setOutput := useTarget(t)
	setOutput(1024)
	waitTarget(t, nginxAddr(t, 2201), nginxAddr(t, 2202))
	pool := nginxAddr(t, 2222)
	server2 := st.Addr(t, "server2", 9100)
	t.Cleanup(func() { st.Healthy(t) }) // server1 is killed

	from := time.Now()
	moved := make(chan string, 1)
	go func() { moved <- killServer1(t.Context(), st, server2, *duration/2) }()
	res := runSessions(t.Context(), t, st, []string{pool}, *sessions, *rate, *duration, true)

	errs := failed(t, res)
	report(t, "L3", "broken sessions", "0", fmt.Sprint(errs), errs == 0)
	m := <-moved
	report(t, "L3", "kill server1 -> werbot_tunnel_streams(server2) = 2", "reference", m, !strings.HasPrefix(m, "kill"))
	s := settle(t, st, resource, from, *sessions)
	if !s.OK {
		t.Logf("not settled: %s", s)
	}
	v := verifyAll(t, st, s.Sessions)
	report(t, "L3", "recordings verified", "reference", fmt.Sprintf("%d of %d", v, *sessions), true)
}

// killServer1 kills server1 after delay and returns how long server2 took to
// carry both tunnel streams, or why that was not seen.
func killServer1(ctx context.Context, st *cluster.Stand, server2 string, delay time.Duration) string {
	select {
	case <-ctx.Done():
		return "kill: " + ctx.Err().Error()
	case <-time.After(delay):
	}
	if _, err := st.Compose(ctx, "kill", "server1"); err != nil {
		return "kill: " + err.Error()
	}
	t0 := time.Now()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for time.Since(t0) < time.Minute {
		if n, err := cluster.ReadMetric(server2, "werbot_tunnel_streams"); err == nil && n == 2 {
			return time.Since(t0).Round(10 * time.Millisecond).String()
		}
		<-tick.C
	}
	return "not within 1m"
}

// TestL4: *sessions idle sessions, half on gw1 and half on gw2 (2 500 each by
// default), held for *idle: gateway RSS within budget in every sample (10 §10.1,
// 2 500 -> 840 MB), and the gateways listen only on SSH and HTTP.
func TestL4(t *testing.T) {
	needStand(t)
	_, setOutput := useTarget(t)
	setOutput(0)
	gw1, gw2 := nginxAddr(t, 2201), nginxAddr(t, 2202)
	waitTarget(t, gw1, gw2)

	ctx, stop := context.WithCancel(t.Context())
	samples := sampleEvery(ctx, t, st, sampleInterval)
	res := runSessions(t.Context(), t, st, []string{gw1, gw2}, *sessions, *rate, *idle, false)
	stop()
	all := <-samples

	// the load itself: every idle session held a slot (gw1 takes the odd one)
	for i, gw := range []string{"gw1", "gw2"} {
		want := int64((*sessions + 1 - i) / 2)
		result, pass := activeRow(all, gw, want)
		report(t, "L4", gw+" peak sessions_active", fmt.Sprint(want), result, pass)
	}
	reportRSS(t, "L4", all, "gw1", gatewayBudget)
	reportRSS(t, "L4", all, "gw2", gatewayBudget)
	report(t, "L4", "errors", "reference", fmt.Sprint(failed(t, res)), true)
	want := []uint16{2222, 9090} // an unused entrance does not listen (10 §10.1)
	for _, gw := range []string{"gw1", "gw2"} {
		got := listenPorts(t, st, gw)
		report(t, "L4", gw+" listeners", "2222, 9090", strings.Trim(fmt.Sprint(got), "[]"), slices.Equal(got, want))
	}
}
