//go:build cluster || load || redteam

package cluster

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

// addKey makes alice's ed25519 key, writes KeyFile and registers the public key.
func (s *Stand) addKey(ctx context.Context) error {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	if s.Key, err = gossh.NewSignerFromKey(priv); err != nil {
		return err
	}
	blk, err := gossh.MarshalPrivateKey(priv, "alice")
	if err != nil {
		return err
	}
	if s.tmp, err = os.MkdirTemp("", "werbot-cluster-"); err != nil {
		return err
	}
	s.KeyFile = filepath.Join(s.tmp, "id_ed25519")
	if err := os.WriteFile(s.KeyFile, pem.EncodeToMemory(blk), 0o600); err != nil {
		return err
	}
	_, err = s.Edge.SSHKeys.AddSSHKey(ctx, connect.NewRequest(&werbotv1.AddSSHKeyRequest{
		Name: "alice", PublicKey: string(gossh.MarshalAuthorizedKey(s.Key.PublicKey())),
	}))
	if err != nil {
		return fmt.Errorf("add ssh key: %w", err)
	}
	return nil
}

// waitSSH takes the pool host key from the first handshake with gw1 (:2201),
// then waits until alice reaches target through gw1 and gw2.
func (s *Stand) waitSSH(ctx context.Context) error {
	gw1, err := s.port(ctx, "nginx", 2201)
	if err != nil {
		return err
	}
	gw2, err := s.port(ctx, "nginx", 2202)
	if err != nil {
		return err
	}
	return poll(ctx, 30*time.Second, func(context.Context) error { // gossh.Dial is bounded by its 10 s Timeout
		if s.HostKey == nil {
			c, err := gossh.Dial("tcp", gw1, s.clientConfig("alice+target", func(_ string, _ net.Addr, key gossh.PublicKey) error {
				s.HostKey = key
				return nil
			}))
			if c != nil {
				_ = c.Close()
			}
			if s.HostKey == nil {
				return fmt.Errorf("host key of %s: %w", gw1, err)
			}
		}
		for _, addr := range []string{gw1, gw2} {
			c, err := s.dial(addr, "alice+target")
			if err != nil {
				return err
			}
			out, code, err := run(c, "hostname")
			_ = c.Close()
			if err != nil || out != "target\n" || code != 0 {
				return fmt.Errorf("hostname via %s: %q, code %d, %v", addr, out, code, err)
			}
		}
		return nil
	})
}

func (s *Stand) clientConfig(user string, hostKey gossh.HostKeyCallback) *gossh.ClientConfig {
	return &gossh.ClientConfig{User: user, Auth: []gossh.AuthMethod{gossh.PublicKeys(s.Key)}, HostKeyCallback: hostKey, Timeout: 10 * time.Second}
}

func (s *Stand) dial(addr, user string) (*gossh.Client, error) {
	return gossh.Dial("tcp", addr, s.clientConfig(user, gossh.FixedHostKey(s.HostKey)))
}

// Dial connects as user with alice's key, pinned to the pool host key; the client closes with the test.
func (s *Stand) Dial(t testing.TB, addr, user string) *gossh.Client {
	t.Helper()
	c, err := s.dial(addr, user)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// run executes cmd and returns stdout+stderr and the exit code; err is set only without an exit status.
func run(c *gossh.Client, cmd string) (string, int, error) {
	sess, err := c.NewSession()
	if err != nil {
		return "", 0, err
	}
	defer sess.Close()
	out, err := sess.CombinedOutput(cmd)
	var ee *gossh.ExitError
	if errors.As(err, &ee) {
		return string(out), ee.ExitStatus(), nil
	}
	return string(out), 0, err
}

// Exec runs cmd on c and returns stdout+stderr and the exit code.
func Exec(t testing.TB, c *gossh.Client, cmd string) (out string, code int) {
	t.Helper()
	out, code, err := run(c, cmd)
	require.NoError(t, err, out)
	return out, code
}

// Shell is an interactive session on an xterm 80x24 PTY.
type Shell struct {
	in   io.Writer
	out  *syncBuffer
	done chan struct{} // closed when the session ends
}

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

// OpenShell starts a login shell on c; the session closes with the test.
func OpenShell(t testing.TB, c *gossh.Client) *Shell {
	t.Helper()
	sess, err := c.NewSession()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sess.Close() })
	require.NoError(t, sess.RequestPty("xterm", 24, 80, gossh.TerminalModes{gossh.ECHO: 1}))
	in, err := sess.StdinPipe()
	require.NoError(t, err)
	sh := &Shell{in: in, out: &syncBuffer{}, done: make(chan struct{})}
	sess.Stdout, sess.Stderr = sh.out, sh.out
	require.NoError(t, sess.Shell())
	go func() {
		_ = sess.Wait()
		close(sh.done)
	}()
	return sh
}

// Mark prints "mark:<mark>" through the shell and waits 5 s for it; mark is one
// plain word (it goes into the command line unquoted). Only the
// command output carries it: the echoed input reads "mark:%s" (the line may
// start with terminal escapes, such as bash's bracketed-paste switch).
func (sh *Shell) Mark(t testing.TB, mark string) {
	t.Helper()
	from := len(sh.out.String())
	_, err := fmt.Fprintf(sh.in, "printf 'mark:%%s\\n' %s\r", mark)
	require.NoError(t, err)
	seen := func() bool { return strings.Contains(sh.out.String()[from:], "mark:"+mark+"\r") }
	if !assert.Eventually(t, seen, 5*time.Second, 50*time.Millisecond) {
		t.Fatalf("mark %q not printed; output:\n%s", mark, sh.out.String()[from:])
	}
}

// Exit types exit and waits 5 s for the session to close.
func (sh *Shell) Exit(t testing.TB) {
	t.Helper()
	_, err := io.WriteString(sh.in, "exit\r")
	require.NoError(t, err)
	select {
	case <-sh.done:
	case <-time.After(5 * time.Second):
		t.Fatalf("shell did not close 5s after exit; output:\n%s", sh.out.String())
	}
}
