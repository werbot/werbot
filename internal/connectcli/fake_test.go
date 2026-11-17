package connectcli

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/platform/sshdgram"
)

func TestMain(m *testing.M) {
	keepaliveInterval, backoffFirst, backoffMax = 200*time.Millisecond, 10*time.Millisecond, 100*time.Millisecond
	os.Exit(m.Run())
}

// fakeGateway is the gateway's SSH front as werbot connect sees it: it accepts
// one key, answers resource@werbot.com with info (refuses when nil), echoes
// direct-tcpip streams (half-close included) and datagram@werbot.com frames.
type fakeGateway struct {
	addr    string
	hostKey gossh.Signer
	info    *sshdgram.ResourceInfo

	down    atomic.Bool  // accepted connections are closed at once
	stall   atomic.Bool  // keepalives get no reply
	dials   atomic.Int64 // connections served
	streams atomic.Int64 // accepted direct-tcpip channels

	mu     sync.Mutex
	conns  []net.Conn
	users  []string
	opens  []sshdgram.OpenData
	refuse string // channel opens are refused with this text
}

func startFake(t *testing.T, client gossh.PublicKey, info *sshdgram.ResourceInfo) *fakeGateway {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	f := &fakeGateway{addr: ln.Addr().String(), hostKey: newKey(t), info: info}
	srv := &ssh.Server{
		PublicKeyHandler: func(_ ssh.Context, k ssh.PublicKey) bool { return ssh.KeysEqual(k, client) },
		ChannelHandlers:  map[string]ssh.ChannelHandler{"direct-tcpip": f.stream, sshdgram.ChannelType: f.datagram},
		RequestHandlers:  map[string]ssh.RequestHandler{sshdgram.ResourceRequest: f.resource, "keepalive@openssh.com": f.keepalive},
	}
	srv.AddHostKey(f.hostKey)
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			if f.down.Load() {
				_ = c.Close()
				continue
			}
			f.dials.Add(1)
			f.mu.Lock()
			f.conns = append(f.conns, c)
			f.mu.Unlock()
			wg.Go(func() { srv.HandleConn(c) })
		}
	})
	t.Cleanup(func() { _ = ln.Close(); f.drop(); wg.Wait() })
	return f
}

// drop closes every open connection, as a killed gateway instance does.
func (f *fakeGateway) drop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.conns {
		_ = c.Close()
	}
	f.conns = nil
}

func (f *fakeGateway) resource(ctx ssh.Context, _ *ssh.Server, _ *gossh.Request) (bool, []byte) {
	f.mu.Lock()
	f.users = append(f.users, ctx.User())
	f.mu.Unlock()
	if f.info == nil {
		return false, nil
	}
	b, err := json.Marshal(f.info)
	return err == nil, b
}

func (f *fakeGateway) keepalive(ctx ssh.Context, _ *ssh.Server, _ *gossh.Request) (bool, []byte) {
	if f.stall.Load() {
		<-ctx.Done()
	}
	return false, nil
}

// accept records the open data and accepts nc unless opens are refused.
func (f *fakeGateway) accept(nc gossh.NewChannel) gossh.Channel {
	var d sshdgram.OpenData
	if err := gossh.Unmarshal(nc.ExtraData(), &d); err != nil {
		_ = nc.Reject(gossh.ConnectionFailed, "bad open data")
		return nil
	}
	f.mu.Lock()
	f.opens = append(f.opens, d)
	refuse := f.refuse
	f.mu.Unlock()
	if refuse != "" {
		_ = nc.Reject(gossh.Prohibited, refuse)
		return nil
	}
	ch, reqs, err := nc.Accept()
	if err != nil {
		return nil
	}
	go gossh.DiscardRequests(reqs)
	return ch
}

func (f *fakeGateway) stream(_ *ssh.Server, _ *gossh.ServerConn, nc gossh.NewChannel, _ ssh.Context) {
	ch := f.accept(nc)
	if ch == nil {
		return
	}
	f.streams.Add(1)
	defer ch.Close() //nolint:errcheck // test server
	_, _ = io.Copy(ch, ch)
	_ = ch.CloseWrite()
}

func (f *fakeGateway) datagram(_ *ssh.Server, _ *gossh.ServerConn, nc gossh.NewChannel, _ ssh.Context) {
	ch := f.accept(nc)
	if ch == nil {
		return
	}
	defer ch.Close() //nolint:errcheck // test server
	buf := make([]byte, sshdgram.MaxPayload)
	for {
		flow, n, err := sshdgram.ReadFrame(ch, buf)
		if err != nil {
			return
		}
		if err := sshdgram.WriteFrame(ch, flow, append([]byte("re:"), buf[:n]...)); err != nil {
			return
		}
	}
}

func (f *fakeGateway) openData() []sshdgram.OpenData {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sshdgram.OpenData(nil), f.opens...)
}

func newKey(t *testing.T) gossh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	s, err := gossh.NewSignerFromKey(priv)
	require.NoError(t, err)
	return s
}

// writeKey writes a new ed25519 key in OpenSSH format to path (encrypted with
// a non-empty passphrase) and returns its signer.
func writeKey(t *testing.T, path, passphrase string) gossh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	var blk *pem.Block
	if passphrase == "" {
		blk, err = gossh.MarshalPrivateKey(priv, "")
	} else {
		blk, err = gossh.MarshalPrivateKeyWithPassphrase(priv, "", []byte(passphrase))
	}
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(blk), 0o600))
	s, err := gossh.NewSignerFromKey(priv)
	require.NoError(t, err)
	return s
}

// syncBuffer is a bytes.Buffer safe for Run's goroutines and the test.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// running is a Run in the background.
type running struct {
	stdout, stderr syncBuffer
	cancel         context.CancelFunc
	done           chan error
}

func startRun(t *testing.T, cfg Config) *running {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	r := &running{cancel: cancel, done: make(chan error, 1)}
	go func() { r.done <- Run(ctx, cfg, &r.stdout, &r.stderr) }()
	t.Cleanup(func() { _ = r.stop(t) })
	return r
}

// stop cancels Run and returns its result; Run must return within 1s.
func (r *running) stop(t *testing.T) error {
	t.Helper()
	r.cancel()
	select {
	case err := <-r.done:
		r.done <- err // a second stop (the cleanup) gets the same
		return err
	case <-time.After(time.Second):
		t.Fatal("Run did not return within 1s of the cancel")
		return nil
	}
}

// wait returns Run's result once it returns by itself.
func (r *running) wait(t *testing.T) error {
	t.Helper()
	select {
	case err := <-r.done:
		r.done <- err
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return within 10s")
		return nil
	}
}

// ready waits for the listening line of resource and returns the local address.
func (r *running) ready(t *testing.T, resource string, port uint16, proto string) string {
	t.Helper()
	line := resource + " → 127.0.0.1:" + strconv.Itoa(int(port)) + " (" + proto + ")\n"
	require.Eventually(t, func() bool { return bytes.Contains([]byte(r.stdout.String()), []byte(line)) },
		5*time.Second, 5*time.Millisecond, "stdout %q, stderr %q", r.stdout.String(), r.stderr.String())
	return "127.0.0.1:" + strconv.Itoa(int(port))
}

// testSetup is a fake gateway for info and a Config of one key for spec.
func testSetup(t *testing.T, info *sshdgram.ResourceInfo, spec string) (*fakeGateway, Config) {
	t.Helper()
	dir := t.TempDir()
	key := writeKey(t, filepath.Join(dir, "id"), "")
	f := startFake(t, key.PublicKey(), info)
	sp, err := ParseSpec(spec)
	require.NoError(t, err)
	return f, Config{Gateway: f.addr, Login: "alice", Identity: filepath.Join(dir, "id"),
		KnownHosts: filepath.Join(dir, "ssh", "known_hosts"), Specs: []Spec{sp}}
}

// freePortNum is a TCP and UDP port free on 127.0.0.1 right now.
func freePortNum(t *testing.T) uint16 {
	t.Helper()
	for {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		port := ln.Addr().(*net.TCPAddr).Port
		pc, err := net.ListenPacket("udp", ln.Addr().String())
		_ = ln.Close()
		if err == nil {
			_ = pc.Close()
			return uint16(port) //nolint:gosec // a port
		}
	}
}
