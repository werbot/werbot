package gateway

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/platform/metrics"
)

// sshPolicies permits every user everything except the resource "denied".
const sshPolicies = `
permit (principal, action, resource);
forbid (principal, action, resource == Werbot::Resource::"rdenied");`

// sshEnv is an SSH front on a pipeline with a fake control plane.
type sshEnv struct {
	f    *fakeControl
	p    *Pipeline
	m    ids
	keys map[string]gossh.Signer // "alice", "bob": users of the same name
	host gossh.Signer
	addr string
	reg  *metrics.Registry
	log  *syncBuffer // the fronts' log
}

func newKey(t *testing.T) gossh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	s, err := gossh.NewSignerFromKey(priv)
	require.NoError(t, err)
	return s
}

func fp(s gossh.Signer) string { return gossh.FingerprintSHA256(s.PublicKey()) }

// newSSHEnv serves the front on 127.0.0.1; cfg.MaxSessions 0 means 10.
func newSSHEnv(t *testing.T, cfg Config) *sshEnv {
	t.Helper()
	if cfg.MaxSessions == 0 {
		cfg.MaxSessions = 10
	}
	m := newIDs("u1", "u2", "rdb", "rweb", "rdenied", "rblocker", "rnope", "rtcp", "rspy")
	e := &sshEnv{m: m, keys: map[string]gossh.Signer{"alice": newKey(t), "bob": newKey(t)}, host: newKey(t), log: &syncBuffer{}}
	e.f = &fakeControl{policy: e.compile(t, res{"rdb", "db", "fake"}, res{"rweb", "web", "fake"}, res{"rdenied", "denied", "fake"},
		res{"rblocker", "blocker", "blocker"}, res{"rnope", "nope", "nope"}, res{"rtcp", "tcp", "streamonly"}, res{"rspy", "spy", "spy"}),
		connected: true, keyErrs: map[string]error{},
		byKey: map[string]User{fp(e.keys["alice"]): {ID: m["u1"], Login: "alice"}, fp(e.keys["bob"]): {ID: m["u2"], Login: "bob"}}}
	e.reg = metrics.NewRegistry()
	cfg.Metrics = e.reg
	e.p = newPipeline(e.f, cfg, slog.New(slog.DiscardHandler))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	e.addr = e.serve(t, e.front(cfg, e.host), ln)
	t.Cleanup(func() { e.p.retries.Wait() })
	return e
}

func (e *sshEnv) compile(t *testing.T, rs ...res) *Policy {
	t.Helper()
	return compileTest(t, e.m, sshPolicies, nil, rs...)
}

func (e *sshEnv) front(cfg Config, host gossh.Signer) *sshFront {
	return newSSHFront(cfg, host, slog.New(slog.NewTextHandler(e.log, nil)))
}

// serve runs front f of e.p on ln until the test ends and returns its address.
func (e *sshEnv) serve(t *testing.T, f *sshFront, ln net.Listener) string {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- f.Serve(ctx, ln, e.p) }()
	t.Cleanup(func() {
		cancel()
		e.p.stopNow() // the front closes its connections once the pipeline stops
		require.NoError(t, <-done)
	})
	return ln.Addr().String()
}

func (e *sshEnv) dial(user string, signers ...gossh.Signer) (*gossh.Client, error) {
	return gossh.Dial("tcp", e.addr, &gossh.ClientConfig{User: user, Auth: []gossh.AuthMethod{gossh.PublicKeys(signers...)},
		HostKeyCallback: gossh.FixedHostKey(e.host.PublicKey()), Timeout: 5 * time.Second})
}

// client logs in as user with alice's key.
func (e *sshEnv) client(t *testing.T, user string) *gossh.Client {
	t.Helper()
	c, err := e.dial(user, e.keys["alice"])
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func (e *sshEnv) startCalls() int {
	var n int
	e.f.with(func(f *fakeControl) { n = f.startCalls })
	return n
}

// run opens a session: pty asks for a 120x40 xterm, cmd "" is a shell. It returns stdout and the exit code.
func run(t *testing.T, c *gossh.Client, pty bool, cmd string) (string, int) {
	t.Helper()
	s, err := c.NewSession()
	require.NoError(t, err)
	defer s.Close() //nolint:errcheck // already closed by the server
	if pty {
		require.NoError(t, s.RequestPty("xterm", 40, 120, gossh.TerminalModes{}))
	}
	var out bytes.Buffer
	s.Stdout = &out
	if cmd == "" {
		require.NoError(t, s.Shell())
	} else {
		require.NoError(t, s.Start(cmd))
	}
	code := exitCode(t, s.Wait())
	return out.String(), code
}

func exitCode(t *testing.T, err error) int {
	t.Helper()
	var ee *gossh.ExitError
	if errors.As(err, &ee) {
		return ee.ExitStatus()
	}
	require.NoError(t, err)
	return 0
}

// badSigner offers a real key, then signs in an unknown format: the server checks
// the key and refuses the signature without dropping the connection, so the
// client goes on with its next key.
type badSigner struct{ gossh.Signer }

func (b badSigner) Sign(r io.Reader, data []byte) (*gossh.Signature, error) {
	s, err := b.Signer.Sign(r, data)
	if err == nil {
		s.Format = "x-unknown@werbot"
	}
	return s, err
}

func TestUnknownKeyRejected(t *testing.T) {
	e := newSSHEnv(t, Config{})
	_, err := e.dial("alice+db", newKey(t))
	require.ErrorContains(t, err, "unable to authenticate")
}

func TestLoginMismatchRejected(t *testing.T) {
	e := newSSHEnv(t, Config{})
	_, err := e.dial("alice+db", e.keys["bob"])
	require.Error(t, err)
	_, err = e.dial("Alice+db", e.keys["alice"])
	require.Error(t, err, "a malformed login is refused")
}

// x/crypto calls the key handler for keys the client only offers; a user found
// for an offered key must never carry over to the key that signs (CVE-2024-45337).
func TestOfferedKeyDoesNotAuthenticate(t *testing.T) {
	t.Run("signing key unverifiable", func(t *testing.T) {
		e := newSSHEnv(t, Config{})
		e.f.with(func(f *fakeControl) { f.keyErrs[fp(e.keys["bob"])] = connect.NewError(connect.CodeUnavailable, nil) })
		c, err := e.dial("alice+db", badSigner{e.keys["alice"]}, e.keys["bob"])
		require.NoError(t, err)
		defer c.Close() //nolint:errcheck // test client
		out, code := run(t, c, false, "uptime")
		require.Equal(t, "Gateway is not ready, try again later\r\n", out)
		require.Equal(t, 1, code)
		require.Zero(t, e.startCalls())
		e.f.with(func(f *fakeControl) {
			require.Contains(t, f.authCalls[0], fp(e.keys["alice"]), "alice's key was offered")
		})

		c, err = e.dial("alice", badSigner{e.keys["alice"]}, e.keys["bob"])
		require.NoError(t, err)
		defer c.Close() //nolint:errcheck // test client
		out, code = menu(t, c, "")
		require.Equal(t, "Gateway is not ready, try again later\r\n", out, "no menu of the offered key's user")
		require.Equal(t, 1, code)
	})
	t.Run("signing key of another user", func(t *testing.T) {
		e := newSSHEnv(t, Config{})
		e.f.with(func(f *fakeControl) { f.byKey[fp(e.keys["bob"])] = User{ID: e.m["u2"], Login: "alice"} })
		c, err := e.dial("alice+db", badSigner{e.keys["alice"]}, e.keys["bob"])
		require.NoError(t, err)
		defer c.Close() //nolint:errcheck // test client
		_, code := run(t, c, false, "uptime")
		require.Equal(t, 0, code)
		e.f.with(func(f *fakeControl) {
			require.Len(t, f.starts, 1)
			require.Contains(t, f.starts[0], e.m["u2"].String(), "the session runs as the signing key's user")
			require.Contains(t, f.authCalls[len(f.authCalls)-1], fp(e.keys["bob"]), "Open authenticates the signing key")
		})
	})
}

func TestNotReadyFailsClosed(t *testing.T) {
	const notReady = "Gateway is not ready, try again later\r\n"
	e := newSSHEnv(t, Config{})
	pol := e.f.Policy()
	e.f.with(func(f *fakeControl) { f.policy = nil })
	out, code := run(t, e.client(t, "alice+db"), false, "uptime")
	require.Equal(t, notReady, out)
	require.Equal(t, 1, code)

	// the control plane is unreachable at the handshake: the key is taken, every channel refused
	e.f.with(func(f *fakeControl) { f.policy, f.authErr = pol, connect.NewError(connect.CodeUnavailable, nil) })
	c := e.client(t, "alice+db")
	for _, cmd := range []string{"", "uptime"} {
		out, code := run(t, c, false, cmd)
		require.Equal(t, notReady, out, cmd)
		require.Equal(t, 1, code, cmd)
	}
	_, err := c.Dial("tcp", "10.0.0.1:80")
	require.ErrorContains(t, err, "prohibited")
	require.Zero(t, e.startCalls())
	require.Contains(t, e.metrics(t), `werbot_gateway_sessions_rejected_total{reason="not_ready"} 4`, "every refusal counts")
}

func (e *sshEnv) metrics(t *testing.T) string {
	t.Helper()
	rec := httptest.NewRecorder()
	e.reg.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	return rec.Body.String()
}

func TestPoolHostKey(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	blk, err := gossh.MarshalPrivateKey(priv, "")
	require.NoError(t, err)
	dir := t.TempDir()
	require.NoError(t, writeAtomic(filepath.Join(dir, hostKeyFile), pem.EncodeToMemory(blk), 0o600))

	e := newSSHEnv(t, Config{})
	var seen []string
	for range 2 {
		hk, err := loadHostKey(dir)
		require.NoError(t, err)
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		addr := e.serve(t, e.front(Config{}, hk), ln)
		c, err := gossh.Dial("tcp", addr, &gossh.ClientConfig{User: "alice+db", Auth: []gossh.AuthMethod{gossh.PublicKeys(e.keys["alice"])},
			HostKeyCallback: func(_ string, _ net.Addr, k gossh.PublicKey) error {
				seen = append(seen, gossh.FingerprintSHA256(k))
				return nil
			}})
		require.NoError(t, err)
		_ = c.Close()
	}
	require.Len(t, seen, 2)
	require.Equal(t, seen[0], seen[1])
}
