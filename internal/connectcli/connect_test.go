package connectcli

import (
	"context"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/platform/sshdgram"
)

var tcpInfo = &sshdgram.ResourceInfo{Type: "tcp", Kinds: []string{"stream"}, Ports: []sshdgram.Port{{Port: 5432}}}

// echoPing sends msg through conn and expects it back.
func echoPing(t *testing.T, conn net.Conn, msg string) {
	t.Helper()
	require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
	_, err := io.WriteString(conn, msg)
	require.NoError(t, err)
	got := make([]byte, len(msg))
	_, err = io.ReadFull(conn, got)
	require.NoError(t, err)
	assert.Equal(t, msg, string(got))
}

func TestConnectTCP(t *testing.T) {
	port := freePortNum(t)
	f, cfg := testSetup(t, tcpInfo, strconv.Itoa(int(port))+":pg")
	r := startRun(t, cfg)
	addr := r.ready(t, "pg", port, "tcp")
	for i := range 3 {
		c, err := net.Dial("tcp", addr)
		require.NoError(t, err)
		echoPing(t, c, "ping"+strconv.Itoa(i))
		_ = c.Close()
	}
	assert.Equal(t, int64(3), f.streams.Load(), "one channel, so one gateway session, per connection")
	assert.Equal(t, int64(1), f.dials.Load(), "one SSH connection per resource")
	for _, d := range f.openData() {
		assert.Equal(t, "pg", d.Host)
		assert.Equal(t, uint32(5432), d.Port)
	}
	f.mu.Lock()
	assert.Equal(t, []string{"alice+pg"}, f.users)
	f.mu.Unlock()
	require.NoError(t, r.stop(t))
	_, err := net.Dial("tcp", addr)
	assert.Error(t, err, "the local port closes with Run")
}

// Each resource of one call gets its own SSH connection and local port.
func TestConnectTwoResources(t *testing.T) {
	p1, p2 := freePortNum(t), freePortNum(t)
	f, cfg := testSetup(t, tcpInfo, strconv.Itoa(int(p1))+":pg")
	cfg.Specs = append(cfg.Specs, Spec{LocalPort: p2, Resource: "pg2"})
	r := startRun(t, cfg)
	for _, a := range []string{r.ready(t, "pg", p1, "tcp"), r.ready(t, "pg2", p2, "tcp")} {
		c, err := net.Dial("tcp", a)
		require.NoError(t, err)
		echoPing(t, c, "ping")
		_ = c.Close()
	}
	assert.Equal(t, int64(2), f.dials.Load())
	f.mu.Lock()
	assert.Equal(t, []string{"alice+pg", "alice+pg2"}, f.users)
	f.mu.Unlock()
}

func TestConnectTCPHalfClose(t *testing.T) {
	port := freePortNum(t)
	_, cfg := testSetup(t, tcpInfo, strconv.Itoa(int(port))+":pg")
	r := startRun(t, cfg)
	c, err := net.Dial("tcp", r.ready(t, "pg", port, "tcp"))
	require.NoError(t, err)
	defer c.Close() //nolint:errcheck // test client
	require.NoError(t, c.SetDeadline(time.Now().Add(5*time.Second)))
	_, err = io.WriteString(c, "request")
	require.NoError(t, err)
	require.NoError(t, c.(*net.TCPConn).CloseWrite())
	got, err := io.ReadAll(c) // the echo ends only after it saw the end of the request
	require.NoError(t, err)
	assert.Equal(t, "request", string(got))
}

func TestConnectUDPTwoFlows(t *testing.T) {
	port := freePortNum(t)
	info := &sshdgram.ResourceInfo{Type: "udp", Kinds: []string{"datagram"}, Ports: []sshdgram.Port{{Port: 53}}}
	f, cfg := testSetup(t, info, strconv.Itoa(int(port))+":dns")
	r := startRun(t, cfg)
	addr := r.ready(t, "dns", port, "udp")
	var socks [2]net.Conn
	for i := range socks {
		c, err := net.Dial("udp", addr)
		require.NoError(t, err)
		defer c.Close() //nolint:errcheck // test client
		socks[i] = c
	}
	for round := range 2 {
		for i, c := range socks {
			msg := "q" + strconv.Itoa(i) + strconv.Itoa(round)
			require.NoError(t, c.SetDeadline(time.Now().Add(5*time.Second)))
			_, err := c.Write([]byte(msg))
			require.NoError(t, err)
			buf := make([]byte, 64)
			n, err := c.Read(buf)
			require.NoError(t, err)
			assert.Equal(t, "re:"+msg, string(buf[:n]))
		}
	}
	d := f.openData()
	require.Len(t, d, 1, "one datagram channel carries both flows")
	assert.Equal(t, sshdgram.OpenData{Host: "dns", Port: 53, OriginAddr: "127.0.0.1", OriginPort: d[0].OriginPort}, d[0])
}

func TestConnectSeveralPorts(t *testing.T) {
	info := &sshdgram.ResourceInfo{Type: "tcp", Kinds: []string{"stream"}, Ports: []sshdgram.Port{{Port: 8000, EndPort: 8002}}}
	_, cfg := testSetup(t, info, "web")
	err := startRun(t, cfg).wait(t)
	assert.EqualError(t, err, "resource web has several ports (8000-8002); use web:<port>")

	port := freePortNum(t)
	f, cfg := testSetup(t, info, strconv.Itoa(int(port))+":web:8001")
	r := startRun(t, cfg)
	c, err := net.Dial("tcp", r.ready(t, "web", port, "tcp"))
	require.NoError(t, err)
	defer c.Close() //nolint:errcheck // test client
	echoPing(t, c, "ping")
	assert.Equal(t, uint32(8001), f.openData()[0].Port)
}

// A spec that fails after others opened ends Run with its error and closes them.
func TestConnectSecondSpecFails(t *testing.T) {
	port := freePortNum(t)
	info := &sshdgram.ResourceInfo{Type: "tcp", Kinds: []string{"stream"}, Ports: []sshdgram.Port{{Port: 8000, EndPort: 8002}}}
	_, cfg := testSetup(t, info, strconv.Itoa(int(port))+":web:8001")
	cfg.Specs = append(cfg.Specs, Spec{Resource: "web"})
	assert.ErrorContains(t, startRun(t, cfg).wait(t), "several ports")
	_, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(int(port)))
	assert.Error(t, err, "the first port is closed")
}

func TestConnectAccessDenied(t *testing.T) {
	_, cfg := testSetup(t, nil, "pg")
	r := startRun(t, cfg)
	err := r.wait(t)
	require.EqualError(t, err, "pg: refused by the gateway")
	assert.Equal(t, "Access denied\n", r.stderr.String())
}

// A refusal with a text prints it as it is.
func TestConnectRefusalText(t *testing.T) {
	f, cfg := testSetup(t, nil, "pg")
	f.mu.Lock()
	f.denial = "Gateway is not ready, try again later"
	f.mu.Unlock()
	r := startRun(t, cfg)
	require.EqualError(t, r.wait(t), "pg: refused by the gateway")
	assert.Equal(t, "Gateway is not ready, try again later\n", r.stderr.String())
}

func TestConnectNoPortAccess(t *testing.T) {
	_, cfg := testSetup(t, &sshdgram.ResourceInfo{Type: "ssh", Kinds: []string{"terminal", "exec"}}, "srv")
	assert.EqualError(t, startRun(t, cfg).wait(t), "Resource srv does not support port access")
}

// A refused channel prints the gateway text as it is and closes the local connection.
func TestConnectChannelRefused(t *testing.T) {
	port := freePortNum(t)
	f, cfg := testSetup(t, tcpInfo, strconv.Itoa(int(port))+":pg")
	f.mu.Lock()
	f.refuse = "Gateway is busy, try again later"
	f.mu.Unlock()
	r := startRun(t, cfg)
	c, err := net.Dial("tcp", r.ready(t, "pg", port, "tcp"))
	require.NoError(t, err)
	defer c.Close() //nolint:errcheck // test client
	require.NoError(t, c.SetDeadline(time.Now().Add(5*time.Second)))
	_, err = c.Read(make([]byte, 1))
	assert.ErrorIs(t, err, io.EOF)
	assert.Equal(t, "Gateway is busy, try again later\n", r.stderr.String())
	assert.NoError(t, r.stop(t))
}

// A refused datagram channel prints its text once and is not opened again for
// the datagrams that follow within the backoff.
func TestConnectUDPRefusedBacksOff(t *testing.T) {
	prev := backoffFirst
	t.Cleanup(func() { backoffFirst = prev }) // runs after Run stopped
	backoffFirst = 300 * time.Millisecond
	port := freePortNum(t)
	info := &sshdgram.ResourceInfo{Type: "udp", Kinds: []string{"datagram"}, Ports: []sshdgram.Port{{Port: 53}}}
	f, cfg := testSetup(t, info, strconv.Itoa(int(port))+":dns")
	f.mu.Lock()
	f.refuse = "Access denied"
	f.mu.Unlock()
	r := startRun(t, cfg)
	c, err := net.Dial("udp", r.ready(t, "dns", port, "udp"))
	require.NoError(t, err)
	defer c.Close() //nolint:errcheck // test client
	send := func() bool { _, err := c.Write([]byte("q")); return err == nil }
	require.Eventually(t, func() bool { return send() && r.stderr.String() != "" }, 5*time.Second, 5*time.Millisecond)
	for range 20 { // inside the backoff: dropped without an open
		send()
	}
	opens := len(f.openData())
	require.Eventually(t, func() bool { return send() && len(f.openData()) > opens }, 5*time.Second, 5*time.Millisecond)
	assert.Equal(t, opens+1, len(f.openData()), "the datagrams sent in the backoff opened nothing")
	assert.Equal(t, "Access denied\n", r.stderr.String(), "the same text is printed once")
}

// Config.Signer replaces the agent and key file, and is asked again on every redial.
func TestConnectSigner(t *testing.T) {
	port := freePortNum(t)
	f, cfg := testSetup(t, tcpInfo, strconv.Itoa(int(port))+":pg")
	raw, err := os.ReadFile(cfg.Identity)
	require.NoError(t, err)
	key, err := gossh.ParsePrivateKey(raw)
	require.NoError(t, err)
	cfg.Identity = ""
	var calls atomic.Int64
	cfg.Signer = func(context.Context) (gossh.Signer, error) { calls.Add(1); return key, nil }
	r := startRun(t, cfg)
	addr := r.ready(t, "pg", port, "tcp")
	assert.Equal(t, int64(1), calls.Load())

	f.down.Store(true)
	f.drop()
	require.Eventually(t, func() bool { return strings.Contains(r.stderr.String(), "reconnecting") }, 5*time.Second, 5*time.Millisecond)
	f.down.Store(false)
	pingEventually(t, addr)
	assert.GreaterOrEqual(t, calls.Load(), int64(2), "a redial asks the signer again")
	assert.NoError(t, r.stop(t))
}
