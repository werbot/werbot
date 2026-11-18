//go:build cluster

package cluster

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tcpv1 "github.com/werbot/werbot/gen/werbot/connector/tcp/v1"
	udpv1 "github.com/werbot/werbot/gen/werbot/connector/udp/v1"
	connectorv1 "github.com/werbot/werbot/gen/werbot/connector/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

const (
	dnsRecovery = 30 * time.Second        // 11 §9: DNS answers again within 30 s of the kill of an instance
	dnsStep     = 1500 * time.Millisecond // ruling R15: at most 20 lookups per wait (client flow table: 32, 2 m)
	lookupMax   = 3 * time.Second
	healthMax   = 3 * time.Minute     // health probes run every 60 s ±20 %
	probeName   = "probe.werbot.test" // Corefile of docker-compose.yaml
	probeAddr   = "10.0.0.1"
)

// TestScenario14Connect: werbot connect carries TCP (echo) and UDP (DNS)
// through the pool front; sessions are stored as stream and datagram; health
// probes the TCP resource from both instances; after the kill of the
// instance that serves the DNS binding, DNS answers again within 30 s.
func TestScenario14Connect(t *testing.T) {
	st.Healthy(t)
	echo := st.AddPortResource(t, "c-echo", &tcpv1.Config{Host: "echo", Ports: []*connectorv1.PortRange{{Port: 7}}})
	dns := st.AddPortResource(t, "c-dns", &udpv1.Config{Host: "dns", Ports: []*connectorv1.PortRange{{Port: 53}}})
	tcpPort, udpPort := freePort(t, "tcp"), freePort(t, "udp") // ruling R6
	c := st.StartConnect(t, "alice", tcpPort+":c-echo", udpPort+":c-dns")
	require.Equal(t, "127.0.0.1:"+tcpPort, c.Ports["c-echo"])
	require.Equal(t, "127.0.0.1:"+udpPort, c.Ports["c-dns"])

	require.NoError(t, echoProbe(c.Ports["c-echo"]))
	for range 2 {
		addrs, err := lookup(c.Ports["c-dns"])
		require.NoError(t, err)
		require.Equal(t, []string{probeAddr}, addrs)
	}

	// ruling C7: the requested target; ruling G8: bytes_up lands when the stream session ends
	const last = `SELECT kind, target, bytes_up FROM session WHERE resource_id = $1 ORDER BY started_at DESC, id DESC LIMIT 1`
	var kind, target string
	var bytesUp int64
	stored := func() bool {
		var n int64
		return st.DB.QueryRow(context.Background(), last, echo).Scan(new(string), new(string), &n) == nil && n > 0
	}
	assert.Eventually(t, stored, 30*time.Second, time.Second, "bytes_up of the c-echo session")
	require.NoError(t, st.DB.QueryRow(t.Context(), last, echo).Scan(&kind, &target, &bytesUp))
	assert.Equal(t, "stream", kind)
	assert.Equal(t, "c-echo:7", target)
	assert.Positive(t, bytesUp)
	require.NoError(t, st.DB.QueryRow(t.Context(), last, dns).Scan(&kind, &target, &bytesUp))
	assert.Equal(t, "datagram", kind)
	assert.Equal(t, "c-dns:53", target)

	// ruling C8: health before the kill
	waitHealth(t, echo)

	// the kill block runs last
	var gw string
	require.NoError(t, st.DB.QueryRow(t.Context(), `
		SELECT gi.hostname FROM session s JOIN gateway_instance gi ON gi.id = s.gateway_instance_id
		WHERE s.resource_id = $1 AND s.ended_at IS NULL ORDER BY s.started_at DESC, s.id DESC LIMIT 1`, dns).Scan(&gw))
	require.Contains(t, []string{"gw1", "gw2"}, gw)
	t.Cleanup(func() {
		_, err := st.Compose(context.Background(), "start", gw)
		assert.NoError(t, err)
		st.Healthy(t)
	})
	t0 := time.Now() // the window includes the docker CLI time of the kill
	_, err := st.Compose(t.Context(), "kill", gw)
	require.NoError(t, err)
	var seen atomic.Value // Eventually runs the condition in its own goroutine
	answers := func() bool {
		addrs, err := lookup(c.Ports["c-dns"])
		seen.Store(fmt.Sprint(addrs, err))
		return err == nil && slices.Equal(addrs, []string{probeAddr})
	}
	if !assert.Eventually(t, answers, dnsRecovery-time.Since(t0), dnsStep, "DNS after the kill of %s", gw) {
		t.Fatalf("last lookup: %v", seen.Load())
	}
	t.Logf("DNS answers %s after the kill of %s", time.Since(t0).Round(10*time.Millisecond), gw)
	echoes := func() bool { err := echoProbe(c.Ports["c-echo"]); seen.Store(fmt.Sprint(err)); return err == nil }
	if !assert.Eventually(t, echoes, 30*time.Second, time.Second, "TCP echo after the kill of %s", gw) {
		t.Fatalf("last echo: %v", seen.Load())
	}
}

// waitHealth waits healthMax until c-echo is healthy, c-dns unknown (UDP is not
// probed) and both instances of the pool reported a probe of c-echo.
func waitHealth(t *testing.T, echo uuid.UUID) {
	t.Helper()
	var last atomic.Value
	probed := func() bool {
		res, err := st.Edge.Resources.ListResources(context.Background(), connect.NewRequest(&werbotv1.ListResourcesRequest{}))
		if err != nil {
			last.Store(err.Error())
			return false
		}
		state := map[string]werbotv1.ResourceHealth_State{}
		for _, r := range res.Msg.GetResources() {
			state[r.GetName()] = r.GetHealth().GetState()
		}
		var n int
		err = st.DB.QueryRow(context.Background(), `SELECT count(DISTINCT gateway_instance_id) FROM resource_health WHERE resource_id = $1`, echo).Scan(&n)
		last.Store(fmt.Sprintf("c-echo %s, c-dns %s, probers %d, %v", state["c-echo"], state["c-dns"], n, err))
		return err == nil && n == 2 && state["c-echo"] == werbotv1.ResourceHealth_STATE_HEALTHY && state["c-dns"] == werbotv1.ResourceHealth_STATE_UNKNOWN
	}
	if !assert.Eventually(t, probed, healthMax, 5*time.Second) {
		t.Fatalf("health: %v", last.Load())
	}
}

// freePort is a free local port of network ("tcp" or "udp"), found by listen and close.
func freePort(t testing.TB, network string) string {
	t.Helper()
	var addr net.Addr
	if network == "udp" {
		pc, err := net.ListenPacket("udp", "127.0.0.1:0")
		require.NoError(t, err)
		addr = pc.LocalAddr()
		require.NoError(t, pc.Close())
	} else {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		addr = ln.Addr()
		require.NoError(t, ln.Close())
	}
	_, port, err := net.SplitHostPort(addr.String())
	require.NoError(t, err)
	return port
}

// echoProbe sends "werbot-probe" through addr, half-closes and expects it back.
func echoProbe(addr string) error {
	c, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return err
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.WriteString(c, "werbot-probe"); err != nil {
		return err
	}
	if err := c.(*net.TCPConn).CloseWrite(); err != nil {
		return err
	}
	got, err := io.ReadAll(c)
	if err != nil {
		return err
	}
	if string(got) != "werbot-probe" {
		return errors.New("echo " + strconv.Quote(string(got)))
	}
	return nil
}

// lookup resolves probeName with the stdlib resolver through the local UDP port addr.
func lookup(addr string) ([]string, error) {
	r := &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "udp", addr)
	}}
	ctx, cancel := context.WithTimeout(context.Background(), lookupMax)
	defer cancel()
	return r.LookupHost(ctx, probeName)
}
