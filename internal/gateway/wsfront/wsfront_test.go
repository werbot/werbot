package wsfront

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// listen starts a plain listener on 127.0.0.1, closed when the test ends.
func listen(t *testing.T, trusted ...netip.Prefix) net.Listener {
	t.Helper()
	ln, err := Listener("127.0.0.1:0", "", "", trusted)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	return ln
}

func wsURL(addr string) string { return "ws://" + addr + Path }

// dialCtx bounds a test dial to 5s.
func dialCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// dial opens a binary WebSocket connection to url with opts.
func dial(t *testing.T, url string, opts *websocket.DialOptions) net.Conn {
	t.Helper()
	c, _, err := websocket.Dial(dialCtx(t), url, opts) //nolint:bodyclose // the 101 body belongs to the conn
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.CloseNow() })
	return websocket.NetConn(t.Context(), c, websocket.MessageBinary)
}

// accept waits for the next connection of ln.
func accept(t *testing.T, ln net.Listener) net.Conn {
	t.Helper()
	type result struct {
		c   net.Conn
		err error
	}
	ch := make(chan result, 1)
	go func() {
		c, err := ln.Accept()
		ch <- result{c, err}
	}()
	select {
	case r := <-ch:
		require.NoError(t, r.err)
		// in the background: the close handshake waits for the client, whose cleanup runs next
		t.Cleanup(func() { go r.c.Close() }) //nolint:errcheck // test conn
		return r.c
	case <-time.After(5 * time.Second):
		t.Fatal("no connection accepted")
		return nil
	}
}

// pass writes msg on from and reads it back on to.
func pass(t *testing.T, from, to net.Conn, msg string) {
	t.Helper()
	_, err := io.WriteString(from, msg)
	require.NoError(t, err)
	buf := make([]byte, len(msg))
	_, err = io.ReadFull(to, buf)
	require.NoError(t, err)
	require.Equal(t, msg, string(buf))
}

// withPing sets pingInterval for the listeners the test starts.
func withPing(t *testing.T, d time.Duration) {
	t.Helper()
	old := pingInterval
	pingInterval = d
	t.Cleanup(func() { pingInterval = old })
}

func TestAcceptUpgradesOnPath(t *testing.T) {
	ln := listen(t)
	client := dial(t, wsURL(ln.Addr().String()), nil)
	server := accept(t, ln)
	pass(t, client, server, "client to server")
	pass(t, server, client, "server to client")
	tr, ok := server.(interface{ Transport() string })
	require.True(t, ok, "the connection reports its transport")
	assert.Equal(t, "ws", tr.Transport())
}

func TestRejectsOtherRequests(t *testing.T) {
	ln := listen(t)
	accepted := make(chan net.Conn, 1)
	go func() {
		if c, err := ln.Accept(); err == nil {
			accepted <- c
		}
	}()
	status := func(method, path string) int {
		req, err := http.NewRequestWithContext(t.Context(), method, "http://"+ln.Addr().String()+path, nil)
		require.NoError(t, err)
		resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		return resp.StatusCode
	}
	assert.Equal(t, http.StatusNotFound, status(http.MethodGet, "/other"), "another path")
	assert.Equal(t, http.StatusMethodNotAllowed, status(http.MethodPost, Path), "another method")
	assert.Equal(t, http.StatusUpgradeRequired, status(http.MethodGet, Path), "no upgrade")

	_, resp, err := websocket.Dial(dialCtx(t), wsURL(ln.Addr().String()),
		&websocket.DialOptions{HTTPHeader: http.Header{"Origin": {"https://other.example"}}})
	require.Error(t, err, "a cross-origin upgrade")
	require.NotNil(t, resp)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	select {
	case <-accepted:
		t.Fatal("a refused request reached Accept")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestTextMessageClosesConn(t *testing.T) {
	ln := listen(t)
	c, _, err := websocket.Dial(dialCtx(t), wsURL(ln.Addr().String()), nil) //nolint:bodyclose // the 101 body belongs to the conn
	require.NoError(t, err)
	defer c.CloseNow() //nolint:errcheck // test client
	server := accept(t, ln)

	closed := make(chan error, 1)
	go func() { _, _, err := c.Read(t.Context()); closed <- err }() // answers the close handshake

	require.NoError(t, c.Write(t.Context(), websocket.MessageText, []byte("hello")))
	_, err = server.Read(make([]byte, 16))
	require.Error(t, err, "a text message is not SSH data")
	assert.Equal(t, websocket.StatusUnsupportedData, websocket.CloseStatus(<-closed), "the connection is closed")
}

func TestForwardedForTrustedOnly(t *testing.T) {
	lo, ten := netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("10.0.0.0/8")
	cases := []struct {
		name     string
		trusted  []netip.Prefix
		xff      string
		want     string
		peerPort bool // the TCP peer's port; 0 for an X-Forwarded-For address
	}{
		{"trusted peer", []netip.Prefix{lo}, "203.0.113.7", "203.0.113.7", false},
		{"rightmost untrusted hop", []netip.Prefix{lo}, "198.51.100.1, 203.0.113.7", "203.0.113.7", false},
		{"no trusted proxies", nil, "203.0.113.7", "127.0.0.1", true},
		{"untrusted peer", []netip.Prefix{ten}, "203.0.113.7", "127.0.0.1", true},
		{"no header", []netip.Prefix{lo}, "", "127.0.0.1", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ln := listen(t, c.trusted...)
			h := http.Header{}
			if c.xff != "" {
				h.Set("X-Forwarded-For", c.xff)
			}
			dial(t, wsURL(ln.Addr().String()), &websocket.DialOptions{HTTPHeader: h})
			addr, ok := accept(t, ln).RemoteAddr().(*net.TCPAddr)
			require.True(t, ok, "RemoteAddr is a *net.TCPAddr")
			assert.Equal(t, c.want, addr.IP.String())
			assert.Equal(t, c.peerPort, addr.Port != 0, "port %d", addr.Port)
		})
	}
}

// writeCert writes a self-signed certificate for 127.0.0.1 and its key into dir.
func writeCert(t *testing.T, dir string) (certFile, keyFile string, roots *x509.CertPool) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)}, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, pub, priv)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	roots = x509.NewCertPool()
	roots.AddCert(cert)
	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err)
	certFile, keyFile = filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600))
	return certFile, keyFile, roots
}

func TestTLS(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile, roots := writeCert(t, dir)
	ln, err := Listener("127.0.0.1:0", certFile, keyFile, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	tlsClient := func(c *tls.Config) *websocket.DialOptions {
		return &websocket.DialOptions{HTTPClient: &http.Client{Transport: &http.Transport{TLSClientConfig: c}}}
	}

	client := dial(t, "wss://"+ln.Addr().String()+Path, tlsClient(&tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}))
	pass(t, client, accept(t, ln), "over tls")

	_, _, err = websocket.Dial(dialCtx(t), wsURL(ln.Addr().String()), nil) //nolint:bodyclose // no response
	require.Error(t, err, "plain ws on the TLS port")
	_, _, err = websocket.Dial(dialCtx(t), "wss://"+ln.Addr().String()+Path, //nolint:bodyclose // no response
		tlsClient(&tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS11})) //nolint:gosec // the old version is the case
	require.Error(t, err, "TLS below 1.2")

	broken := filepath.Join(dir, "broken.pem")
	require.NoError(t, os.WriteFile(broken, []byte("not a pem"), 0o600))
	_, err = Listener("127.0.0.1:0", broken, keyFile, nil)
	require.Error(t, err, "an unreadable certificate fails at start")
	_, err = Listener("127.0.0.1:0", certFile, filepath.Join(dir, "missing.pem"), nil)
	require.Error(t, err, "a missing key fails at start")
}

func TestCloseKeepsAcceptedConns(t *testing.T) {
	ln := listen(t)
	addr := ln.Addr().String()
	client := dial(t, wsURL(addr), nil)
	server := accept(t, ln)

	require.NoError(t, ln.Close())
	_, err := ln.Accept()
	require.ErrorIs(t, err, net.ErrClosed)
	_, _, err = websocket.Dial(dialCtx(t), wsURL(addr), nil) //nolint:bodyclose // no response
	require.Error(t, err, "a closed listener takes no new connections")

	pass(t, client, server, "after close")
	pass(t, server, client, "still open")
}

// One message above the default 32 KiB read limit (OpenSSH sends ~35 KB SSH
// packets) reaches the reader whole.
func TestLargeMessage(t *testing.T) {
	ln := listen(t)
	client := dial(t, wsURL(ln.Addr().String()), nil)
	server := accept(t, ln)
	msg := strings.Repeat("x", 1<<20)
	go func() { _, _ = io.WriteString(client, msg) }() // one Write is one message
	buf := make([]byte, len(msg))
	_, err := io.ReadFull(server, buf)
	require.NoError(t, err)
	require.Equal(t, msg, string(buf))
}
