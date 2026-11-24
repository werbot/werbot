//go:build unix

package app

import (
	"context"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/werbot/werbot/internal/connectcli"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/platform/metrics"
)

// startWSGateway runs a second instance of the pool "gw" with the WebSocket
// input on, configured from the WERBOT_GATEWAY_* variables, and returns its
// ws:// URL (no path: the client adds /v1/ssh).
func (s *stack) startWSGateway(t *testing.T) string {
	t.Helper()
	wsAddr := freeAddr(t)
	cfg, err := gateway.LoadConfig(nil, env(map[string]string{
		"WERBOT_GATEWAY_SERVER_URL": s.url, "WERBOT_GATEWAY_JOIN_TOKEN": s.token, "WERBOT_GATEWAY_DATA_DIR": t.TempDir(),
		"WERBOT_GATEWAY_SSH_ADDR": freeAddr(t), "WERBOT_GATEWAY_NAME": "gw-ws", "WERBOT_GATEWAY_DRAIN_TIMEOUT": "1s",
		"WERBOT_GATEWAY_WS_ADDR": wsAddr, "WERBOT_GATEWAY_TRUSTED_PROXIES": "10.0.0.0/8",
	}))
	require.NoError(t, err)
	require.NoError(t, runUntilUp(t, func(ctx context.Context, reg *metrics.Registry) error {
		cfg.Metrics = reg
		return gateway.Run(ctx, cfg, slog.New(slog.DiscardHandler))
	}))
	return "ws://" + wsAddr
}

// wsConfig is a werbot connect config for spec through the gateway at wsURL only.
func wsConfig(t *testing.T, wsURL, keyFile, spec string) connectcli.Config {
	t.Helper()
	sp, err := connectcli.ParseSpec(spec)
	require.NoError(t, err)
	return connectcli.Config{WSURL: wsURL, Login: "alice", Identity: keyFile,
		KnownHosts: filepath.Join(t.TempDir(), "known_hosts"), Specs: []connectcli.Spec{sp}}
}

// werbot connect with only the WebSocket URL: TCP and UDP reach the resources,
// the session is stored with transport ws and the peer's address, and the
// gateway host key is remembered under the URL's host:port.
func TestConnectOverWebSocket(t *testing.T) {
	s := startStack(t)
	keyFile := s.addKeyFile(t)
	pg := s.addPortResource(t, "pg", "tcp", startTCPEcho(t), true)
	s.addPortResource(t, "dns", "udp", startUDPEcho(t), true)
	s.resourceReady(t, "pg")
	s.resourceReady(t, "dns")
	wsURL := s.startWSGateway(t)

	t.Run("tcp", func(t *testing.T) {
		port := freePort(t)
		cfg := wsConfig(t, wsURL, keyFile, port+":pg")
		r := runConnect(t, cfg)
		c, err := net.Dial("tcp", r.ready(t, "pg", port, "tcp"))
		require.NoError(t, err)
		require.NoError(t, c.SetDeadline(time.Now().Add(5*time.Second)))
		echoPing(t, c)
		require.NoError(t, c.Close())
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			var transport, ip string
			require.NoError(c, s.pool.QueryRow(context.Background(),
				`SELECT transport, host(client_ip) FROM session WHERE resource_id = $1 AND kind = 'stream'`, pg).Scan(&transport, &ip))
			assert.Equal(c, "ws", transport)
			assert.Equal(c, "127.0.0.1", ip, "no trusted proxy in front: the TCP peer")
		}, 10*time.Second, 100*time.Millisecond)
		kh, err := os.ReadFile(cfg.KnownHosts)
		require.NoError(t, err)
		addr := connectcli.HostKeyAddr(connectcli.Endpoint{WSURL: wsURL})
		assert.True(t, strings.HasPrefix(string(kh), knownhosts.Normalize(addr)+" "), string(kh))
		assert.NoError(t, r.stop(t))
	})

	t.Run("udp two flows", func(t *testing.T) {
		port := freePort(t)
		addr := runConnect(t, wsConfig(t, wsURL, keyFile, port+":dns")).ready(t, "dns", port, "udp")
		for i := range 2 {
			c, err := net.Dial("udp", addr)
			require.NoError(t, err)
			t.Cleanup(func() { _ = c.Close() })
			msg := "q" + strconv.Itoa(i)
			require.NoError(t, c.SetDeadline(time.Now().Add(5*time.Second)))
			_, err = c.Write([]byte(msg))
			require.NoError(t, err)
			buf := make([]byte, 64)
			n, err := c.Read(buf)
			require.NoError(t, err)
			assert.Equal(t, "re:"+msg, string(buf[:n]))
		}
	})
}

// A gateway host key other than the one known for the URL's host:port is
// refused over WebSocket as over SSH; known_hosts is left as it was.
func TestConnectOverWebSocketWrongHostKey(t *testing.T) {
	s := startStack(t)
	keyFile := s.addKeyFile(t)
	s.addPortResource(t, "pg", "tcp", startTCPEcho(t), true)
	s.resourceReady(t, "pg")
	wsURL := s.startWSGateway(t)
	cfg := wsConfig(t, wsURL, keyFile, "pg")
	addr := connectcli.HostKeyAddr(connectcli.Endpoint{WSURL: wsURL})
	line := knownhosts.Line([]string{knownhosts.Normalize(addr)}, newSigner(t).PublicKey()) + "\n"
	require.NoError(t, os.WriteFile(cfg.KnownHosts, []byte(line), 0o600))

	err := runConnect(t, cfg).wait(t)
	assert.ErrorContains(t, err, "pg: ssh: handshake failed: host key mismatch for "+addr)
	kh, err := os.ReadFile(cfg.KnownHosts)
	require.NoError(t, err)
	assert.Equal(t, line, string(kh))
}

// The keyboard-interactive MFA round after the key works over WebSocket: a
// TOTP code typed at the prompt opens the resource.
func TestConnectOverWebSocketMFA(t *testing.T) {
	m := mfaSSH(t)
	keyFile := m.addKeyFile(t)
	code := totpAt(t, enrollTOTP(t, m.web), 0)
	wsURL := m.startWSGateway(t)
	_, echoPort, err := net.SplitHostPort(m.target.EchoAddr)
	require.NoError(t, err)
	port := freePort(t)
	cfg := wsConfig(t, wsURL, keyFile, port+":srv:"+echoPort)
	cfg.Stdin = strings.NewReader(code + "\n")

	r := runConnect(t, cfg)
	c, err := net.Dial("tcp", r.ready(t, "srv", port, "tcp"))
	require.NoError(t, err)
	require.NoError(t, c.SetDeadline(time.Now().Add(5*time.Second)))
	echoPing(t, c)
	require.NoError(t, c.Close())
	assert.Contains(t, r.stderr.String(), "or enter TOTP")
	assert.NoError(t, r.stop(t))
}
