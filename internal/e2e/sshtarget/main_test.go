package main

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

const testPassword = "p@ss"

// dialTarget serves newServer on a free local port and connects with testPassword.
func dialTarget(t *testing.T) *gossh.Client {
	t.Helper()
	srv, err := newServer("127.0.0.1:0", testPassword)
	require.NoError(t, err)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	c, err := gossh.Dial("tcp", ln.Addr().String(), &gossh.ClientConfig{
		User: "werbot", Auth: []gossh.AuthMethod{gossh.Password(testPassword)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec // ephemeral host key of the test server
		Timeout:         5 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// runSplit runs cmd and returns its stdout and stderr; chunks are the reads of stdout.
func runSplit(t *testing.T, c *gossh.Client, cmd string) (stdout, stderr string, chunks []string) {
	t.Helper()
	sess, err := c.NewSession()
	require.NoError(t, err)
	defer sess.Close()
	out, err := sess.StdoutPipe()
	require.NoError(t, err)
	var errOut bytes.Buffer
	sess.Stderr = &errOut
	require.NoError(t, sess.Start(cmd))
	var all bytes.Buffer
	buf := make([]byte, 64)
	for {
		n, err := out.Read(buf)
		if n > 0 {
			chunks = append(chunks, string(buf[:n]))
			all.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
	}
	require.NoError(t, sess.Wait())
	return all.String(), errOut.String(), chunks
}

func TestSSHTargetSecretCommands(t *testing.T) {
	c := dialTarget(t)

	stdout, stderr, _ := runSplit(t, c, "cat /var/tmp/secret")
	assert.Equal(t, testPassword+"\n", stdout, "one write of the password")
	assert.Empty(t, stderr)

	stdout, stderr, chunks := runSplit(t, c, "cat /var/tmp/secret.split")
	assert.Equal(t, testPassword+"\n", stdout, "two writes of the password")
	assert.Empty(t, stderr)
	require.GreaterOrEqual(t, len(chunks), 2, "the password arrives in two writes")
	assert.Less(t, len(chunks[0]), len(testPassword), "the first write cuts the value")

	stdout, stderr, _ = runSplit(t, c, "cat /var/tmp/secret.err")
	assert.Empty(t, stdout)
	assert.Equal(t, testPassword+"\n", stderr, "the password on stderr")

	stdout, _, _ = runSplit(t, c, "echo hi")
	assert.Equal(t, "hi\n", stdout, "echo is unchanged")
}
