//go:build cluster

package cluster

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// endLostStartupGrace is session.EndLostStartupGrace: test/ imports no internal package.
const endLostStartupGrace = 2 * time.Minute

// TestScenario03SharedHostKeyStrictKnownHosts: one known_hosts line with the
// pool host key passes strict checking of OpenSSH on gw1, gw2 and the pool
// front; a foreign key fails it.
func TestScenario03SharedHostKeyStrictKnownHosts(t *testing.T) {
	st.Healthy(t)
	gw1, gw2, pool := st.Addr(t, "nginx", 2201), st.Addr(t, "nginx", 2202), st.Addr(t, "nginx", 2222)
	kh := writeKnownHosts(t, []string{gw1, gw2, pool}, st.HostKey)

	instance := map[string]string{}
	for _, addr := range []string{gw1, gw2, pool} {
		out, errOut, code := openSSH(t, kh, addr, "alice+target", "hostname")
		require.Equal(t, 0, code, "ssh via %s: %s", addr, errOut)
		require.Equal(t, "target\n", out, "ssh via %s", addr)
		_, _, instance[addr] = st.LastSession(t, st.Resources["target"])
	}
	assert.Equal(t, "gw1", instance[gw1], "session via :2201")
	assert.Equal(t, "gw2", instance[gw2], "session via :2202")

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	foreign, err := gossh.NewPublicKey(pub)
	require.NoError(t, err)
	_, errOut, code := openSSH(t, writeKnownHosts(t, []string{gw1}, foreign), gw1, "alice+target", "hostname")
	assert.Equal(t, 255, code)
	assert.Contains(t, errOut, "Host key verification failed")
}

// TestScenario04KillReplicaKeepsSession: a session on gw1 survives the kill of
// server1 (its tunnel replica), gw1 moves to server2 within ReconnectMax and
// the recording reaches S3 through server2.
func TestScenario04KillReplicaKeepsSession(t *testing.T) {
	st.Healthy(t)
	t.Cleanup(func() { st.Healthy(t) })
	sh := OpenShell(t, st.Dial(t, st.Addr(t, "nginx", 2201), "alice+target")) // gw1 -> server1
	sh.Mark(t, "before-kill")
	id := st.OpenSessionOn(t, "gw1")
	server2 := st.Addr(t, "server2", 9100)

	_, err := st.Compose(t.Context(), "kill", "server1")
	require.NoError(t, err)
	t0 := time.Now()
	moved := func() bool { n, err := metric(server2, "werbot_tunnel_streams"); return err == nil && n == 2 }
	require.Eventually(t, moved, ReconnectMax, 100*time.Millisecond, "gw1 reconnects to server2")
	t.Logf("gw1 on server2 after %s", time.Since(t0))

	sh.Mark(t, "after-kill")
	sh.Exit(t)
	t.Logf("closed after %s", st.WaitEnded(t, id, "closed", UploadDrain))
	t0 = time.Now()
	st.WaitVerified(t, id, UploadDrain)
	t.Logf("verified after %s", time.Since(t0))
}

// TestScenario05KillInstanceEndsSessionsLost: session.end_lost ends the
// sessions of a killed instance with gateway_lost within LostMax and leaves
// the sessions of the live instance open.
func TestScenario05KillInstanceEndsSessionsLost(t *testing.T) {
	st.Healthy(t)
	t.Cleanup(func() { st.Healthy(t) })
	st.waitUptime(t, endLostStartupGrace, "server1", "server2") // ruling G6: end_lost skips its first runs after a replica start

	keep := OpenShell(t, st.Dial(t, st.Addr(t, "nginx", 2201), "alice+target"))
	keep.Mark(t, "gw1-open")
	keepID := st.OpenSessionOn(t, "gw1")
	sh := OpenShell(t, st.Dial(t, st.Addr(t, "nginx", 2202), "alice+target"))
	sh.Mark(t, "gw2-open")
	id := st.OpenSessionOn(t, "gw2")

	_, err := st.Compose(t.Context(), "kill", "gw2")
	require.NoError(t, err)
	t.Logf("lost after %s", st.WaitEnded(t, id, "gateway_lost", LostMax))

	keep.Mark(t, "gw1-alive")
	var open bool
	require.NoError(t, st.DB.QueryRow(t.Context(), `SELECT ended_at IS NULL FROM session WHERE id = $1`, keepID).Scan(&open))
	assert.True(t, open, "end_lost leaves the session of gw1 open")
}

// TestScenario11MaxSessionsBusy: an instance at WERBOT_GATEWAY_MAX_SESSIONS
// turns /readyz to 503 and refuses the next session as busy until one ends.
func TestScenario11MaxSessionsBusy(t *testing.T) {
	st.Healthy(t)
	t.Cleanup(func() { st.Healthy(t) }) // recreates gw2 with the Up environment (10000)
	ctx := context.Background()
	_, err := st.ComposeEnv(ctx, map[string]string{"GW2_MAX_SESSIONS": "1"}, "up", "-d", "gw2")
	require.NoError(t, err)
	var gw2 string // the published port changes with the new container
	ready := func() bool {
		addr, err := st.port(ctx, "gw2", 9090)
		if err != nil {
			return false
		}
		gw2 = addr
		return Ready(addr) == http.StatusOK
	}
	t0 := time.Now()
	require.Eventually(t, ready, time.Minute, 500*time.Millisecond, "gw2 ready with its identity from gw2-data")
	t.Logf("gw2 ready after %s", time.Since(t0))

	s1 := OpenShell(t, st.Dial(t, st.Addr(t, "nginx", 2202), "alice+target"))
	s1.Mark(t, "one")
	assert.Equal(t, http.StatusServiceUnavailable, Ready(gw2), "/readyz at the limit")

	out, code := Exec(t, st.Dial(t, st.Addr(t, "nginx", 2202), "alice+target"), "hostname")
	assert.Contains(t, out, "Gateway is busy, try again later")
	assert.Equal(t, 1, code)
	assert.Equal(t, int64(1), Metric(t, gw2, "werbot_gateway_sessions_rejected_total", "reason", "busy"))

	s1.Exit(t)
	t0 = time.Now()
	require.Eventually(t, func() bool { return Ready(gw2) == http.StatusOK }, 5*time.Second, 100*time.Millisecond, "/readyz after the session ends")
	t.Logf("ready again after %s", time.Since(t0))
}

// writeKnownHosts writes one known_hosts line for addrs ("127.0.0.1:N") and key into a temp file.
func writeKnownHosts(t *testing.T, addrs []string, key gossh.PublicKey) string {
	t.Helper()
	kh := filepath.Join(t.TempDir(), "known_hosts")
	require.NoError(t, os.WriteFile(kh, []byte(knownhosts.Line(addrs, key)+"\n"), 0o600))
	return kh
}

// openSSH runs cmd as user through the OpenSSH client with strict host key
// checking against kh and alice's key; it returns stdout, stderr and the exit code.
func openSSH(t *testing.T, kh, addr, user, cmd string) (stdout, stderr string, code int) {
	t.Helper()
	host, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, "ssh", "-F", "/dev/null", "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes", //nolint:gosec // fixed binary, arguments from the harness
		"-o", "StrictHostKeyChecking=yes", "-o", "UserKnownHostsFile="+kh, "-i", st.KeyFile, "-p", port,
		user+"@"+host, cmd)
	var o, e bytes.Buffer
	c.Stdout, c.Stderr = &o, &e
	err = c.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return o.String(), e.String(), ee.ExitCode()
	}
	require.NoError(t, err, e.String())
	return o.String(), e.String(), 0
}

// waitUptime waits until every service container has run for at least d since its StartedAt.
func (s *Stand) waitUptime(t *testing.T, d time.Duration, services ...string) {
	t.Helper()
	var last atomic.Value
	up := func() bool {
		for _, svc := range services {
			c, err := s.inspect(context.Background(), svc)
			if err != nil || !c.Running {
				last.Store(fmt.Sprintf("%s: running %t, %v", svc, c.Running, err))
				return false
			}
			if u := time.Since(c.StartedAt); u < d {
				last.Store(fmt.Sprintf("%s: uptime %s", svc, u.Round(time.Second)))
				return false
			}
		}
		return true
	}
	t0 := time.Now()
	if !assert.Eventually(t, up, d+30*time.Second, time.Second) {
		t.Fatalf("uptime of %v below %s: %v", services, d, last.Load())
	}
	t.Logf("waited %s for uptime %s", time.Since(t0).Round(time.Second), d)
}
