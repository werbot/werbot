//go:build load

package load

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/werbot/werbot/gen/gateway/v1/gatewayv1connect"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/test/cluster"
)

var (
	instances = flag.Int("load.instances", 200, "simulated gateway instances (L2)")
	resources = flag.Int("load.resources", 10000, "resources and bindings in the org (L2)")
)

const (
	enrollParallel = 20               // also the batch size of the stream ramp
	firstWait      = 2 * time.Minute  // every sim of a batch has its first snapshot
	versionWait    = 30 * time.Second // every sim has the new version (10 §8.3)
	versionP99     = 5 * time.Second  // 10 §8.3
)

// TestL2: *instances sims enrolled by one join token (max_uses 1000) in pool
// "sim" with *resources resources, each bound to alice; a new binding reaches
// every sim within 30 s, with p99 ≤ 5 s (10 §8.3).
func TestL2(t *testing.T) {
	needStand(t)
	ctx := t.Context()
	n := *instances
	pool, err := st.Edge.Gateways.CreateGateway(ctx, connect.NewRequest(&werbotv1.CreateGatewayRequest{Name: "sim"}))
	require.NoError(t, err)
	poolID := pool.Msg.GetGateway().GetId()
	// one statement, one transaction (ruling G11)
	_, err = st.DB.Exec(ctx, `
		WITH r AS (
			INSERT INTO resource (org_id, name, type, category, gateway_id, config)
			SELECT $1::uuid, 'sim-' || to_char(i, 'FM00000'), 'ssh', 'servers', $2::uuid, '{"host":"10.255.0.1","port":22,"login":"x"}'
			FROM generate_series(1, $3::int) AS i RETURNING id)
		INSERT INTO access_binding (org_id, subject_type, subject_id, resource_id, created_by)
		SELECT $1::uuid, 'user', $4::uuid, id, $4::uuid FROM r`,
		st.OrgID, poolID, *resources, st.AliceID)
	require.NoError(t, err)
	tok, err := st.Edge.Gateways.CreateJoinToken(ctx, connect.NewRequest(&werbotv1.CreateJoinTokenRequest{
		GatewayId: poolID, MaxUses: 1000, ExpiresIn: durationpb.New(time.Hour),
	}))
	require.NoError(t, err)

	sims, clients := enrollSims(ctx, t, tok.Msg.GetToken(), nginxAddr(8443), n)
	assert.EqualValues(t, n, st.TokenUses(t, poolID, tok.Msg.GetId()), "join token uses")

	sctx, stop := context.WithCancel(ctx)
	samples := sampleEvery(sctx, t, st, sampleInterval)
	rctx, cancel := context.WithCancel(ctx)
	var running sync.WaitGroup
	defer running.Wait()
	defer cancel()
	// connect in batches, each waiting for its first snapshots: every Connect builds a
	// full snapshot server-side, so a flood of n at once only measures the connect storm
	first := make([]snap, 0, n)
	for lo := 0; lo < n; lo += enrollParallel {
		batch := sims[lo:min(lo+enrollParallel, n)]
		for i, s := range batch {
			running.Go(func() {
				s.run(rctx, clients[lo+i])
				s.tr.CloseIdleConnections()
			})
		}
		first = append(first, await(t, batch, make([]string, len(batch)), time.Now().Add(firstWait))...)
	}
	var firstLat []time.Duration
	for i, x := range first {
		if x.at.IsZero() {
			continue
		}
		firstLat = append(firstLat, x.at.Sub(sims[i].start))
	}
	got, pass := receivedRow(len(firstLat), n)
	report(t, "L2", "first snapshot", fmt.Sprintf("all %d ≤ %s", n, firstWait), got, pass)
	report(t, "L2", "first snapshot p99", "reference", p99(firstLat).Round(time.Millisecond).String(), len(firstLat) > 0) // ruling P13
	reportStreams(t, n)

	prev := make([]string, n)
	for i, s := range sims {
		prev[i] = drain(s.got, first[i].version)
	}
	res := lookupResource(ctx, t, "sim-00001")
	t0 := time.Now()
	_, err = st.Edge.Access.CreateBinding(ctx, connect.NewRequest(&werbotv1.CreateBindingRequest{
		SubjectType: "user", SubjectId: st.AliceID.String(), ResourceId: res.String(),
	}))
	require.NoError(t, err)
	var lat []time.Duration
	size := 0
	for i, x := range await(t, sims, prev, t0.Add(versionWait)) {
		if !x.at.IsZero() && !first[i].at.IsZero() { // without a first snapshot prev is unknown
			lat, size = append(lat, x.at.Sub(t0)), max(size, x.size)
		}
	}
	got, pass = receivedRow(len(lat), n)
	report(t, "L2", "new version received", fmt.Sprintf("all %d ≤ %s", n, versionWait), got, pass)
	got, pass = versionRow(lat, n)
	report(t, "L2", "version p99", fmt.Sprintf("≤ %s", versionP99), got, pass)
	var top time.Duration
	if len(lat) > 0 {
		top = slices.Max(lat)
	}
	report(t, "L2", "version max", "reference", top.Round(time.Millisecond).String(), len(lat) > 0)
	report(t, "L2", "snapshot size", "reference", mb(int64(size)), size > 0) // ruling R17

	stop()
	all := <-samples
	for _, svc := range []string{"server1", "server2"} {
		got, m, _ := rssRow(all, svc, serverBudget)
		report(t, "L2", "RSS "+svc, "reference", got, m > 0) // ruling R17
	}
	reportSlow(t, "L2", st)
}

// enrollSims enrolls n sims, enrollParallel at a time; every enrollment must succeed.
func enrollSims(ctx context.Context, t testing.TB, token, tunnelAddr string, n int) ([]*sim, []gatewayv1connect.TunnelServiceClient) {
	t.Helper()
	sims, clients, errs := make([]*sim, n), make([]gatewayv1connect.TunnelServiceClient, n), make([]error, n)
	var wg sync.WaitGroup
	sem := make(chan struct{}, enrollParallel)
	for i := range n {
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			sims[i], clients[i], errs[i] = enrollSim(ctx, st, token, tunnelAddr, i)
		})
	}
	wg.Wait()
	require.NoError(t, errors.Join(errs...))
	return sims, clients
}

// await waits for every sim, in parallel, for a snapshot with a version other
// than prev[i] by deadline; a sim without one gets a zero snap and a log line.
func await(t testing.TB, sims []*sim, prev []string, deadline time.Time) []snap {
	t.Helper()
	out, errs := make([]snap, len(sims)), make([]error, len(sims))
	var wg sync.WaitGroup
	for i, s := range sims {
		wg.Go(func() { out[i], errs[i] = s.next(prev[i], deadline) })
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		t.Logf("sims without a new snapshot: %v", err)
	}
	return out
}

// receivedRow passes when all n sims (n > 0) received.
func receivedRow(received, n int) (string, bool) {
	return fmt.Sprintf("%d of %d", received, n), n > 0 && received == n
}

// versionRow is the L2 criterion: p99 of the latencies ≤ versionP99, and every
// one of the n sims measured; a sim that never saw the version fails the row.
func versionRow(lat []time.Duration, n int) (string, bool) {
	p := p99(lat)
	got := p.Round(time.Millisecond).String()
	if len(lat) != n {
		got += fmt.Sprintf(" (%d of %d)", len(lat), n)
	}
	return got, n > 0 && len(lat) == n && p > 0 && p <= versionP99
}

// reportStreams waits up to firstWait for n+2 tunnel streams (the sims, gw1 and
// gw2) on the two server replicas; the split between them is for reference.
func reportStreams(t testing.TB, n int) {
	t.Helper()
	a, b := st.Addr(t, "server1", 9100), st.Addr(t, "server2", 9100)
	var s1, s2 int64
	var err error
	ok := assert.Eventually(t, func() bool {
		if s1, err = cluster.ReadMetric(a, "werbot_tunnel_streams"); err != nil {
			return false
		}
		s2, err = cluster.ReadMetric(b, "werbot_tunnel_streams")
		return err == nil && s1+s2 == int64(n+2)
	}, firstWait, time.Second)
	got := fmt.Sprintf("%d + %d", s1, s2)
	if err != nil {
		got = err.Error()
	}
	report(t, "L2", "werbot_tunnel_streams server1 + server2", fmt.Sprint(n+2), got, ok)
}

func lookupResource(ctx context.Context, t testing.TB, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, st.DB.QueryRow(ctx, `SELECT id FROM resource WHERE org_id = $1 AND name = $2`, st.OrgID, name).Scan(&id))
	return id
}
