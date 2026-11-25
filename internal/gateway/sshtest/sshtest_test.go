package sshtest

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

func TestTargetCommands(t *testing.T) {
	tg := StartTarget(t, "", nil)
	c, err := gossh.Dial("tcp", tg.Addr, &gossh.ClientConfig{User: "u", HostKeyCallback: gossh.FixedHostKey(tg.HostKey), Timeout: 5 * time.Second})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	exec := func(cmd string) (string, error) {
		s, err := c.NewSession()
		require.NoError(t, err)
		defer s.Close() //nolint:errcheck // closed by the target
		var out bytes.Buffer
		s.Stdout = &out
		err = s.Run(cmd)
		return out.String(), err
	}
	out, err := exec("uname")
	require.NoError(t, err)
	require.Equal(t, "Linux\n", out)
	_, err = exec("sleep 0")
	require.NoError(t, err)

	for range 2 { // the sessions above
		require.Equal(t, "closed", <-tg.Events)
	}
	s, err := c.NewSession()
	require.NoError(t, err)
	require.NoError(t, s.Start("sleep 30"))
	time.Sleep(50 * time.Millisecond)
	require.NoError(t, c.Close()) // the client leaves
	select {
	case e := <-tg.Events:
		require.Equal(t, "closed", e)
	case <-time.After(5 * time.Second):
		t.Fatal("sleep goes on after the client left")
	}
}
