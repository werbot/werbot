package gateway

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

// fwdSSH is an SSH env with the resources of fwdPolicy, every one permitted.
func fwdSSH(t *testing.T, cfg Config) *sshEnv {
	t.Helper()
	e := newSSHEnv(t, cfg)
	e.f.setPolicy(fwdPolicy(t, e.m, "v1", sshPolicies, true))
	return e
}

// fwdMsg is the payload of tcpip-forward and cancel-tcpip-forward.
type fwdMsg struct {
	Host string
	Port uint32
}

// forwardedMsg is the payload of a forwarded-tcpip channel.
type forwardedMsg struct {
	Addr       string
	Port       uint32
	OriginAddr string
	OriginPort uint32
}

// sendForward sends the global request typ for host:port and returns the answer.
func sendForward(t *testing.T, c *gossh.Client, typ, host string, port uint32) bool {
	t.Helper()
	ok, _, err := c.SendRequest(typ, true, gossh.Marshal(fwdMsg{host, port}))
	require.NoError(t, err)
	return ok
}

// loadForward is the running fake forward on port.
func loadForward(t *testing.T, port uint32) *RemoteForward {
	t.Helper()
	var rf *RemoteForward
	require.Eventually(t, func() bool {
		v, ok := fwdTargets.Load(port)
		rf, _ = v.(*RemoteForward)
		return ok
	}, 2*time.Second, 5*time.Millisecond)
	return rf
}

// requireAlive checks that the client connection still answers.
func requireAlive(t *testing.T, c *gossh.Client) {
	t.Helper()
	_, _, err := c.SendRequest("keepalive@openssh.com", true, nil)
	require.NoError(t, err)
}

// endReasons are the end reasons of every session so far.
func (e *sshEnv) endReasons() []string {
	var out []string
	e.f.with(func(f *fakeControl) {
		for _, end := range f.ends {
			out = append(out, end.reason)
		}
	})
	return out
}

func requireAllEnds(t *testing.T, e *sshEnv, want ...string) {
	t.Helper()
	require.Eventually(t, func() bool { return len(e.endReasons()) >= len(want) }, 2*time.Second, 5*time.Millisecond)
	require.Equal(t, want, e.endReasons())
}

// Each connection at the target reaches the client as a forwarded-tcpip
// channel of the one session, counted; closing the connection ends it.
func TestRemoteForwardRoundTrip(t *testing.T) {
	e := fwdSSH(t, Config{})
	c := e.client(t, "alice+fwd")
	ln, err := c.Listen("tcp", "127.0.0.1:8081")
	require.NoError(t, err)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { defer conn.Close(); _, _ = io.Copy(conn, conn) }()
		}
	}()
	rf := loadForward(t, 8081)
	for range 2 {
		st, err := rf.Open("127.0.0.1", 40000)
		require.NoError(t, err)
		_, err = io.WriteString(st, "ping")
		require.NoError(t, err)
		buf := make([]byte, 4)
		_, err = io.ReadFull(st, buf)
		require.NoError(t, err)
		require.Equal(t, "ping", string(buf))
		require.NoError(t, st.Close())
	}
	require.Equal(t, 1, e.startCalls())
	require.NoError(t, c.Close())
	requireAllEnds(t, e, "closed")
	e.f.with(func(f *fakeControl) {
		require.EqualValues(t, 8, f.ends[0].counters.GetBytesUp())
		require.EqualValues(t, 8, f.ends[0].counters.GetBytesDown())
	})
}

// Only a loopback bind host and a port from 1 are accepted, once per address.
func TestRemoteForwardRejectsPortZeroAndHostnames(t *testing.T) {
	e := fwdSSH(t, Config{})
	c := e.client(t, "alice+fwd")
	for _, host := range []string{"example.com", "*", "0.0.0.0", "::", "10.0.0.1", "LOCALHOST "} {
		require.False(t, sendForward(t, c, tcpipForward, host, 8090), host)
	}
	require.False(t, sendForward(t, c, tcpipForward, "localhost", 0), "port 0")
	ok, _, err := c.SendRequest(tcpipForward, true, []byte{1, 2})
	require.NoError(t, err)
	require.False(t, ok, "a malformed request")
	require.Zero(t, e.startCalls())

	for i, host := range []string{"127.0.0.1", "::1", "", "localhost"} {
		require.True(t, sendForward(t, c, tcpipForward, host, uint32(8100+i)), host)
	}
	require.False(t, sendForward(t, c, tcpipForward, "127.0.0.1", 8100), "the same address again")
	require.Equal(t, 4, e.startCalls())
}

// The forwarded-tcpip channel names the bind address as the client sent it.
func TestRemoteForwardEchoesBindAddress(t *testing.T) {
	e := fwdSSH(t, Config{})
	c := e.client(t, "alice+fwd")
	chans := c.HandleChannelOpen("forwarded-tcpip")
	require.True(t, sendForward(t, c, tcpipForward, "", 8200))
	rf := loadForward(t, 8200)
	opened := make(chan error, 1)
	go func() {
		st, err := rf.Open("127.0.0.1", 40000)
		if err == nil {
			_ = st.Close()
		}
		opened <- err
	}()
	var nc gossh.NewChannel
	select {
	case nc = <-chans:
	case <-time.After(2 * time.Second):
		t.Fatal("no forwarded-tcpip channel")
	}
	var got forwardedMsg
	require.NoError(t, gossh.Unmarshal(nc.ExtraData(), &got))
	require.Equal(t, forwardedMsg{Addr: "", Port: 8200, OriginAddr: "127.0.0.1", OriginPort: 40000}, got)
	ch, reqs, err := nc.Accept()
	require.NoError(t, err)
	go gossh.DiscardRequests(reqs)
	defer ch.Close()
	require.NoError(t, <-opened)
}

// A cancel ends the forward ("closed") and keeps the connection; an unknown
// address is refused; the address can be forwarded again.
func TestCancelRemoteForward(t *testing.T) {
	e := fwdSSH(t, Config{})
	c := e.client(t, "alice+fwd")
	require.True(t, sendForward(t, c, tcpipForward, "localhost", 8300))
	loadForward(t, 8300)
	require.True(t, sendForward(t, c, cancelTCPIPForward, "localhost", 8300))
	_, running := fwdTargets.Load(uint32(8300))
	require.False(t, running, "the target stopped listening before the answer")
	requireAllEnds(t, e, "closed")
	require.False(t, sendForward(t, c, cancelTCPIPForward, "localhost", 8300))
	require.False(t, sendForward(t, c, cancelTCPIPForward, "127.0.0.1", 8301))
	requireAlive(t, c)
	require.True(t, sendForward(t, c, tcpipForward, "localhost", 8300))
}

func TestRemoteForwardMenuLogin(t *testing.T) {
	e := fwdSSH(t, Config{})
	c := e.client(t, "alice")
	require.False(t, sendForward(t, c, tcpipForward, "localhost", 8080))
	require.Zero(t, e.startCalls())
}

// A target that refuses, or does not listen in time, gets false; the
// connection stays.
func TestRemoteForwardTargetFails(t *testing.T) {
	old := remoteForwardReady
	remoteForwardReady = 300 * time.Millisecond
	t.Cleanup(func() { remoteForwardReady = old })
	e := fwdSSH(t, Config{})
	c := e.client(t, "alice+fwd")
	require.False(t, sendForward(t, c, tcpipForward, "localhost", fwdRefused))
	requireAllEnds(t, e, "error")
	start := time.Now()
	require.False(t, sendForward(t, c, tcpipForward, "localhost", fwdSilent))
	require.Less(t, time.Since(start), 2*time.Second)
	requireAllEnds(t, e, "error", "error")
	requireAlive(t, c)
}

// A forward the gateway ends closes the client connection.
func TestRemoteForwardRevokedClosesConnection(t *testing.T) {
	e := fwdSSH(t, Config{})
	startSweeper(t, e.p)
	c := e.client(t, "alice+fwd")
	require.True(t, sendForward(t, c, tcpipForward, "localhost", 8400))
	loadForward(t, 8400)
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	e.f.setPolicy(fwdPolicy(t, e.m, "v2", `permit (principal, action, resource == Werbot::Resource::"rblocker");`, true))
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the connection outlived the revoked forward")
	}
	requireAllEnds(t, e, "access_revoked")
}

// A running forward keeps the connection past the connection idle limit;
// without it the connection idles out.
func TestRemoteForwardHoldsConnIdle(t *testing.T) {
	const idle = 300 * time.Millisecond
	e := fwdSSH(t, Config{IdleTimeout: idle})
	c := e.client(t, "alice+fwd")
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	require.True(t, sendForward(t, c, tcpipForward, "localhost", 8500))
	select {
	case <-done:
		t.Fatal("the connection idled out under a running forward")
	case <-time.After(4 * idle):
	}
	require.True(t, sendForward(t, c, cancelTCPIPForward, "localhost", 8500))
	select {
	case <-done:
	case <-time.After(5 * idle):
		t.Fatal("the connection without a forward is still open")
	}
}
