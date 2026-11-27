package gateway

import (
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// withPreResources replaces the snapshot of e with the preconn resources "pre",
// "prehandled", "predenied" and "denied" (forbidden).
func (e *sshEnv) withPreResources(t *testing.T) {
	t.Helper()
	rs := []res{{"rdenied", "denied", "preconn"}}
	for _, n := range []string{"pre", "prehandled", "predenied"} {
		e.m["r"+n] = uuid.New()
		rs = append(rs, res{"r" + n, n, "preconn"})
	}
	e.f.setPolicy(e.compile(t, rs...))
}

// A PDP refusal rejects the channel before it is accepted or read.
func TestPreSessionNotCalledWhenDenied(t *testing.T) {
	e := newSSHEnv(t, Config{})
	e.withPreResources(t)
	before := preCalls.Load()
	_, err := e.client(t, "alice+denied").Dial("tcp", "db:5432")
	require.ErrorContains(t, err, `administratively prohibited ("Access denied")`)
	require.Equal(t, before, preCalls.Load())
	require.Zero(t, e.startCalls())
}

// A pre-session connector gets the channel accepted in Open: the peeked bytes
// reach Serve; a handled or refused start closes the accepted channel.
func TestDirectTCPIPPreSession(t *testing.T) {
	e := newSSHEnv(t, Config{})
	e.withPreResources(t)
	c, err := e.client(t, "alice+pre").Dial("tcp", "db:5432")
	require.NoError(t, err)
	_, err = io.WriteString(c, "12345678tail")
	require.NoError(t, err)
	got := make([]byte, 12)
	_, err = io.ReadFull(c, got)
	require.NoError(t, err)
	require.Equal(t, "12345678tail", string(got), "echo from the first byte")
	require.NoError(t, c.Close())
	require.Eventually(t, func() bool { return e.startCalls() == 1 }, 2*time.Second, 10*time.Millisecond)

	for _, name := range []string{"prehandled", "predenied"} {
		c, err := e.client(t, "alice+"+name).Dial("tcp", "db:5432")
		require.NoError(t, err, "%s: accepted before the start was read", name)
		_, err = io.ReadAll(c)
		require.NoError(t, err, "%s: closed without data", name)
	}
	require.Equal(t, 1, e.startCalls(), "no session for a handled or refused start")
	var reasons []string
	require.Eventually(t, func() bool { // Open reports the refusal after it closed the channel
		for _, d := range e.p.heartbeat().GetDenials() {
			reasons = append(reasons, d.GetReason())
		}
		return len(reasons) > 0
	}, 2*time.Second, 10*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	for _, d := range e.p.heartbeat().GetDenials() {
		reasons = append(reasons, d.GetReason())
	}
	require.Equal(t, []string{"denied"}, reasons, "only the refusal is a denial")
}

// A connector without PreSession keeps the 6b order: the channel is accepted
// after Open, and a refused target host is a rejected channel.
func TestDirectTCPIPWithoutPreSession(t *testing.T) {
	e := newSSHEnvWith(t, Config{}, func(p *Pipeline) { p.guard = localGuard{deny: true, hosts: []string{"10.0.0.5"}} })
	e.withPortResources(t)
	_, err := e.client(t, "alice+pg").Dial("tcp", "10.0.0.5:5432")
	require.ErrorContains(t, err, "administratively prohibited")
	require.Zero(t, e.startCalls())

	c, err := e.client(t, "alice+tcp").Dial("tcp", "10.0.0.1:80")
	require.NoError(t, err)
	require.NoError(t, c.Close())
	require.Equal(t, 1, e.startCalls(), "accepted after StartSession")
}
