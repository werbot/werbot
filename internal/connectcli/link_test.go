package connectcli

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// pingEventually dials addr until a ping echoes, within 10s.
func pingEventually(t *testing.T, addr string) {
	t.Helper()
	require.Eventually(t, func() bool {
		c, err := net.Dial("tcp", addr)
		if err != nil {
			return false
		}
		defer c.Close() //nolint:errcheck // test client
		_ = c.SetDeadline(time.Now().Add(2 * time.Second))
		if _, err := c.Write([]byte("ping")); err != nil {
			return false
		}
		b := make([]byte, 4)
		n, _ := c.Read(b)
		return string(b[:n]) == "ping"
	}, 10*time.Second, 20*time.Millisecond)
}

// A dropped gateway connection is dialed again behind the same local port;
// redials that fail back off until the gateway accepts again.
func TestConnectReconnects(t *testing.T) {
	port := freePortNum(t)
	f, cfg := testSetup(t, tcpInfo, strconv.Itoa(int(port))+":pg")
	r := startRun(t, cfg)
	addr := r.ready(t, "pg", port, "tcp")
	pingEventually(t, addr)

	f.down.Store(true)
	f.drop()
	require.Eventually(t, func() bool {
		return strings.Contains(r.stderr.String(), "pg: connection to the gateway lost; reconnecting\n")
	},
		5*time.Second, 5*time.Millisecond)
	f.down.Store(false)
	pingEventually(t, addr)
	assert.Contains(t, r.stderr.String(), "pg: reconnected\n")
	assert.GreaterOrEqual(t, f.dials.Load(), int64(2))
	assert.Equal(t, 1, strings.Count(r.stdout.String(), "→"), "the listener stays the same")
	assert.NoError(t, r.stop(t))
}

// Ctrl+C while the gateway is gone ends Run within 1s (stop asserts it) with nil.
func TestConnectInterruptDuringReconnect(t *testing.T) {
	port := freePortNum(t)
	f, cfg := testSetup(t, tcpInfo, strconv.Itoa(int(port))+":pg")
	r := startRun(t, cfg)
	r.ready(t, "pg", port, "tcp")
	f.down.Store(true)
	f.drop()
	require.Eventually(t, func() bool { return strings.Contains(r.stderr.String(), "reconnecting") }, 5*time.Second, 5*time.Millisecond)
	assert.NoError(t, r.stop(t))
}

// Three keepalive intervals without a reply drop the connection and dial again.
func TestConnectKeepaliveMisses(t *testing.T) {
	port := freePortNum(t)
	f, cfg := testSetup(t, tcpInfo, strconv.Itoa(int(port))+":pg")
	f.stall.Store(true)
	r := startRun(t, cfg)
	addr := r.ready(t, "pg", port, "tcp")
	require.Eventually(t, func() bool { return strings.Contains(r.stderr.String(), "reconnecting") },
		10*time.Second, 10*time.Millisecond)
	f.stall.Store(false)
	pingEventually(t, addr)
	assert.GreaterOrEqual(t, f.dials.Load(), int64(2))
}

// A new local connection waits reconnectWait for a redial, then gives up.
func TestLinkGetWaits(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := newLink(nil, "", nil, "pg", &syncBuffer{})
		l.set(nil)
		start := time.Now()
		_, err := l.get(context.Background())
		assert.ErrorIs(t, err, errNotConnected)
		assert.Equal(t, reconnectWait, time.Since(start))

		c := &gossh.Client{}
		go func() { time.Sleep(time.Second); l.set(c) }()
		got, err := l.get(context.Background())
		require.NoError(t, err)
		assert.Same(t, c, got)
	})
}

func TestConnectHostKeyMismatch(t *testing.T) {
	f, cfg := testSetup(t, tcpInfo, "pg")
	other := newKey(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(cfg.KnownHosts), 0o700))
	line := knownhosts.Line([]string{knownhosts.Normalize(f.addr)}, other.PublicKey())
	require.NoError(t, os.WriteFile(cfg.KnownHosts, []byte(line+"\n"), 0o600))
	err := startRun(t, cfg).wait(t)
	assert.ErrorContains(t, err, "host key mismatch for "+f.addr)
	assert.Equal(t, int64(0), f.streams.Load())
}

// An unknown gateway is added to known_hosts (created 0700/0600) once, then trusted.
func TestConnectKnownHostsAppend(t *testing.T) {
	port := freePortNum(t)
	f, cfg := testSetup(t, tcpInfo, strconv.Itoa(int(port))+":pg")
	r := startRun(t, cfg)
	r.ready(t, "pg", port, "tcp")
	require.NoError(t, r.stop(t))
	b, err := os.ReadFile(cfg.KnownHosts)
	require.NoError(t, err)
	assert.Equal(t, knownhosts.Line([]string{knownhosts.Normalize(f.addr)}, f.hostKey.PublicKey())+"\n", string(b))
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(cfg.KnownHosts)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm())
		di, err := os.Stat(filepath.Dir(cfg.KnownHosts))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o700), di.Mode().Perm())
	}

	r = startRun(t, cfg) // the saved key is trusted, nothing is added
	r.ready(t, "pg", port, "tcp")
	require.NoError(t, r.stop(t))
	b2, err := os.ReadFile(cfg.KnownHosts)
	require.NoError(t, err)
	assert.Equal(t, b, b2)
}
