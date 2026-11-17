package app

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	tcpv1 "github.com/werbot/werbot/gen/werbot/connector/tcp/v1"
	udpv1 "github.com/werbot/werbot/gen/werbot/connector/udp/v1"
	connectorv1 "github.com/werbot/werbot/gen/werbot/connector/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/connectcli"
	"github.com/werbot/werbot/internal/platform/sshdgram"
)

// werbot connect against the real gateway of the e2e stack.
func TestE2EConnect(t *testing.T) {
	s := startStack(t)
	keyFile := s.addKeyFile(t)
	tcpEcho, udpEcho := startTCPEcho(t), startUDPEcho(t)
	pg := s.addPortResource(t, "pg", "tcp", tcpEcho, true)
	s.addPortResource(t, "dns", "udp", udpEcho, true)
	s.addPortResource(t, "closed", "tcp", tcpEcho, false)
	s.resourceReady(t, "pg")
	s.resourceReady(t, "dns")
	cfg := func(gw, spec string) connectcli.Config {
		sp, err := connectcli.ParseSpec(spec)
		require.NoError(t, err)
		return connectcli.Config{Gateway: gw, Login: "alice", Identity: keyFile,
			KnownHosts: filepath.Join(t.TempDir(), "known_hosts"), Specs: []connectcli.Spec{sp}}
	}

	t.Run("tcp", func(t *testing.T) {
		port := freePort(t)
		r := runConnect(t, cfg(s.gwSSH, port+":pg"))
		addr := r.ready(t, "pg", port, "tcp")
		for range 3 {
			c, err := net.Dial("tcp", addr)
			require.NoError(t, err)
			echoPing(t, c)
			require.NoError(t, c.Close())
		}
		c, err := net.Dial("tcp", addr) // half-close: the echo ends after the end of the request
		require.NoError(t, err)
		require.NoError(t, c.SetDeadline(time.Now().Add(5*time.Second)))
		_, err = io.WriteString(c, "request")
		require.NoError(t, err)
		require.NoError(t, c.(*net.TCPConn).CloseWrite())
		got, err := io.ReadAll(c)
		require.NoError(t, err)
		assert.Equal(t, "request", string(got))
		require.NoError(t, c.Close())
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			var n int
			require.NoError(c, s.pool.QueryRow(context.Background(),
				`SELECT count(*) FROM session WHERE resource_id = $1 AND kind = 'stream'`, pg).Scan(&n))
			assert.Equal(c, 4, n, "one session per local connection")
		}, 10*time.Second, 100*time.Millisecond)
		assert.NoError(t, r.stop(t))
	})

	t.Run("udp two flows", func(t *testing.T) {
		port := freePort(t)
		addr := runConnect(t, cfg(s.gwSSH, port+":dns")).ready(t, "dns", port, "udp")
		var socks [2]net.Conn
		for i := range socks {
			c, err := net.Dial("udp", addr)
			require.NoError(t, err)
			t.Cleanup(func() { _ = c.Close() })
			socks[i] = c
		}
		for i, c := range socks {
			msg := "q" + strconv.Itoa(i)
			require.NoError(t, c.SetDeadline(time.Now().Add(5*time.Second)))
			_, err := c.Write([]byte(msg))
			require.NoError(t, err)
			buf := make([]byte, 64)
			n, err := c.Read(buf)
			require.NoError(t, err)
			assert.Equal(t, "re:"+msg, string(buf[:n]))
		}
	})

	t.Run("ssh resource name is the target itself", func(t *testing.T) {
		s.addResource(t, "srv", "ssh") // port_forwarding: true
		s.resourceReady(t, "srv")
		_, echoPort, err := net.SplitHostPort(s.target.EchoAddr)
		require.NoError(t, err)
		port := freePort(t)
		r := runConnect(t, cfg(s.gwSSH, port+":srv:"+echoPort))
		c, err := net.Dial("tcp", r.ready(t, "srv", port, "tcp"))
		require.NoError(t, err)
		echoPing(t, c)
		require.NoError(t, c.Close())
		assert.NoError(t, r.stop(t))
	})

	t.Run("access denied", func(t *testing.T) {
		r := runConnect(t, cfg(s.gwSSH, "closed"))
		assert.EqualError(t, r.wait(t), "closed: refused by the gateway")
		assert.Equal(t, "Access denied\n", r.stderr.String())
	})

	t.Run("reconnect", func(t *testing.T) {
		p := startCutProxy(t, s.gwSSH)
		port := freePort(t)
		r := runConnect(t, cfg(p.addr, port+":pg"))
		addr := r.ready(t, "pg", port, "tcp")
		c, err := net.Dial("tcp", addr)
		require.NoError(t, err)
		echoPing(t, c)
		p.cut()
		_, err = c.Read(make([]byte, 1))
		assert.Error(t, err, "active connections end with the SSH connection")
		require.Eventually(t, func() bool {
			c, err := net.Dial("tcp", addr)
			if err != nil {
				return false
			}
			defer c.Close() //nolint:errcheck // test client
			_ = c.SetDeadline(time.Now().Add(5 * time.Second))
			_, _ = io.WriteString(c, "ping")
			b := make([]byte, 4)
			_, err = io.ReadFull(c, b)
			return err == nil && string(b) == "ping"
		}, 10*time.Second, 50*time.Millisecond)
		assert.Contains(t, r.stderr.String(), "pg: connection to the gateway lost; reconnecting\n")
		assert.NoError(t, r.stop(t))
	})
}

// addKeyFile adds a new key of alice and writes its private key file.
func (s *stack) addKeyFile(t *testing.T) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := gossh.NewSignerFromKey(priv)
	require.NoError(t, err)
	_, err = s.api.keys.AddSSHKey(context.Background(), connect.NewRequest(&werbotv1.AddSSHKeyRequest{
		PublicKey: string(gossh.MarshalAuthorizedKey(signer.PublicKey()))}))
	require.NoError(t, err)
	blk, err := gossh.MarshalPrivateKey(priv, "")
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "id_ed25519")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(blk), 0o600))
	return path
}

// addPortResource creates a tcp or udp resource on 127.0.0.1 at the port of
// addr in the pool "gw", bound to alice when bind is set.
func (s *stack) addPortResource(t *testing.T, name, typ, addr string, bind bool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	_, ps, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	p, err := strconv.ParseUint(ps, 10, 16)
	require.NoError(t, err)
	ports := []*connectorv1.PortRange{{Port: uint32(p)}}
	var cfg proto.Message = &tcpv1.Config{Host: "127.0.0.1", Ports: ports}
	if typ == "udp" {
		cfg = &udpv1.Config{Host: "127.0.0.1", Ports: ports}
	}
	a, err := anypb.New(cfg)
	require.NoError(t, err)
	res, err := s.api.res.CreateResource(ctx, connect.NewRequest(&werbotv1.CreateResourceRequest{
		Name: name, Type: typ, GatewayId: s.gatewayID.String(), Config: a}))
	require.NoError(t, err)
	id := res.Msg.GetResource().GetId()
	if bind {
		_, err = s.api.acc.CreateBinding(ctx, connect.NewRequest(&werbotv1.CreateBindingRequest{
			SubjectType: "user", SubjectId: s.alice, ResourceId: id}))
		require.NoError(t, err)
	}
	return uuid.MustParse(id)
}

// resourceReady waits until the gateway answers resource@werbot.com for name.
func (s *stack) resourceReady(t *testing.T, name string) {
	t.Helper()
	require.Eventually(t, func() bool {
		c, _, err := s.dial(s.gwSSH, "alice+"+name, s.key)
		if err != nil {
			return false
		}
		defer c.Close() //nolint:errcheck // probe
		ok, _, err := c.SendRequest(sshdgram.ResourceRequest, true, nil)
		return err == nil && ok
	}, settle, 100*time.Millisecond)
}

func freePort(t *testing.T) string {
	t.Helper()
	_, p, err := net.SplitHostPort(freeAddr(t))
	require.NoError(t, err)
	return p
}
