//go:build load

package load

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/test/cluster"
)

// gatewayBudget is the RSS budget of a gateway instance (10 §10.1) in decimal
// units: 2 500 sessions -> 840 000 000. It holds for every session kind; a UDP
// session with 32 flows takes about 512 KiB (stage 6b), the L1 mix uses one flow per binding.
func gatewayBudget(active int64) int64 { return 200_000_000 + 256_000*active }

// serverBudget is the RSS budget of a server replica (10 §10.1); streams are
// the tunnel streams plus the relay streams (browsers and bridges).
func serverBudget(streams int64) int64 { return 300_000_000 + 64_000*streams }

// sampled are the services of every sample: gateways report werbot_gateway_sessions_active
// on :9090, server replicas werbot_tunnel_streams plus werbot_relay_streams on :9100.
var sampled = []struct {
	service, metric, plus string
	port                  int
}{
	{"gw1", "werbot_gateway_sessions_active", "", 9090},
	{"gw2", "werbot_gateway_sessions_active", "", 9090},
	{"server1", "werbot_tunnel_streams", "werbot_relay_streams", 9100},
	{"server2", "werbot_tunnel_streams", "werbot_relay_streams", 9100},
}

// sample is one moment of the stand. A service missing from a map could not be
// read at that moment (a killed container).
type sample struct {
	At      time.Time
	RSS     map[string]int64 // VmRSS, bytes
	Active  map[string]int64 // gw: sessions_active, server: tunnel_streams
	PGConns map[string]int64 // Postgres connections per server replica
}

// procRSS is the VmRSS of pid, the main process of a container (docker inspect).
func procRSS(pid int) (int64, error) {
	if pid <= 0 {
		return 0, fmt.Errorf("pid %d: not running", pid)
	}
	f, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return parseVMRSS(f)
}

// parseVMRSS reads "VmRSS: <n> kB" of /proc/<pid>/status; procfs kB are 1024 bytes.
func parseVMRSS(r io.Reader) (int64, error) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "VmRSS:"); ok {
			kb, err := strconv.ParseInt(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(v), "kB")), 10, 64)
			return kb * 1024, err
		}
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("no VmRSS")
}

// sampleEvery reads the stand every sampleInterval until ctx ends, then sends all
// samples and closes the channel. PIDs and addresses are resolved once, now:
// a service killed later drops out of the samples instead of failing them.
func sampleEvery(ctx context.Context, t testing.TB, st *cluster.Stand) <-chan []sample {
	t.Helper()
	pids, addrs := map[string]int{}, map[string]string{}
	for _, s := range sampled {
		pids[s.service] = st.Container(t, s.service).PID
		addrs[s.service] = st.Addr(t, s.service, s.port)
	}
	out := make(chan []sample, 1)
	go func() {
		defer close(out)
		var all []sample
		tick := time.NewTicker(sampleInterval)
		defer tick.Stop()
		for {
			all = append(all, takeSample(ctx, st, pids, addrs))
			select {
			case <-ctx.Done():
				out <- all
				return
			case <-tick.C:
			}
		}
	}()
	return out
}

func takeSample(ctx context.Context, st *cluster.Stand, pids map[string]int, addrs map[string]string) sample {
	s := sample{At: time.Now(), RSS: map[string]int64{}, Active: map[string]int64{}, PGConns: map[string]int64{}}
	for _, svc := range sampled {
		rss, err := procRSS(pids[svc.service])
		if err != nil {
			continue
		}
		active, err := cluster.ReadMetric(addrs[svc.service], svc.metric)
		if err != nil {
			continue
		}
		if svc.plus != "" {
			extra, err := cluster.ReadMetric(addrs[svc.service], svc.plus)
			if err != nil {
				continue
			}
			active += extra
		}
		s.RSS[svc.service], s.Active[svc.service] = rss, active
	}
	rows, err := st.DB.Query(ctx, `
		SELECT host(client_addr), count(*) FROM pg_stat_activity
		WHERE datname = 'werbot' AND host(client_addr) = ANY($1::text[]) GROUP BY 1`,
		[]string{cluster.IPServer1, cluster.IPServer2})
	if err != nil {
		return s
	}
	defer rows.Close()
	replica := map[string]string{cluster.IPServer1: "server1", cluster.IPServer2: "server2"}
	for rows.Next() {
		var ip string
		var n int64
		if rows.Scan(&ip, &n) == nil {
			s.PGConns[replica[ip]] = n
		}
	}
	return s
}

// p99 is the nearest-rank 99th percentile: sorted[ceil(0.99*n) - 1]; 0 for no values.
func p99(d []time.Duration) time.Duration {
	if len(d) == 0 {
		return 0
	}
	s := slices.Clone(d)
	slices.Sort(s)
	return s[(99*len(s)+99)/100-1]
}

// listenPorts are the TCP ports the service's network namespace listens on (IPv4 and IPv6).
func listenPorts(t testing.TB, st *cluster.Stand, service string) []uint16 {
	t.Helper()
	pid := st.Container(t, service).PID
	var files []io.Reader
	for _, name := range []string{"tcp", "tcp6"} {
		f, err := os.Open(fmt.Sprintf("/proc/%d/net/%s", pid, name))
		require.NoError(t, err)
		defer f.Close()
		files = append(files, f)
	}
	ports, err := parseListen(files...)
	require.NoError(t, err)
	return ports
}

// dockerDNS is 127.0.0.11 as /proc/net/tcp prints it (little-endian hex):
// Docker's embedded DNS listens in every container of a user network.
const dockerDNS = "0B00007F"

// parseListen reads /proc/net/tcp{,6} tables and returns the sorted, unique
// local ports in state 0A (LISTEN), without Docker's embedded DNS.
func parseListen(tables ...io.Reader) ([]uint16, error) {
	var ports []uint16
	for _, r := range tables {
		sc := bufio.NewScanner(r)
		sc.Scan() // header
		for sc.Scan() {
			f := strings.Fields(sc.Text())
			if len(f) < 4 || f[3] != "0A" {
				continue
			}
			addr, port, ok := strings.Cut(f[1], ":")
			if !ok {
				return nil, fmt.Errorf("local address %q", f[1])
			}
			if addr == dockerDNS {
				continue
			}
			p, err := strconv.ParseUint(port, 16, 16)
			if err != nil {
				return nil, err
			}
			ports = append(ports, uint16(p))
		}
		if err := sc.Err(); err != nil {
			return nil, err
		}
	}
	slices.Sort(ports)
	return slices.Compact(ports), nil
}

// report prints one result line for test/load/README.md and fails the test when pass is false.
func report(t testing.TB, load, metric, criterion, result string, pass bool) {
	t.Helper()
	fmt.Println(resultLine(load, metric, criterion, result, pass))
	if !pass {
		t.Errorf("%s %s: %s, want %s", load, metric, result, criterion)
	}
}

func resultLine(load, metric, criterion, result string, pass bool) string {
	verdict := "PASS"
	if !pass {
		verdict = "FAIL"
	}
	return strings.Join([]string{"RESULT", load, metric, criterion, result, verdict}, " | ")
}

// mb formats bytes in decimal megabytes, the unit of the budgets.
func mb(b int64) string { return fmt.Sprintf("%.1f MB", float64(b)/1e6) }
