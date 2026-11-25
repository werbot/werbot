package wsfront

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// proxy forwards TCP to a target like an L7 balancer would: a direction that
// carries no bytes for idle closes the connection (0: never), and a frozen
// proxy drops every byte without closing.
type proxy struct {
	ln     net.Listener
	idle   time.Duration
	frozen atomic.Bool
}

func newProxy(t *testing.T, target string, idle time.Duration) *proxy {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	p := &proxy{ln: ln, idle: idle}
	go func() {
		for {
			a, err := ln.Accept()
			if err != nil {
				return
			}
			b, err := net.Dial("tcp", target)
			if err != nil {
				_ = a.Close()
				continue
			}
			go p.pipe(a, b)
			go p.pipe(b, a)
		}
	}()
	return p
}

func (p *proxy) pipe(src, dst net.Conn) {
	defer dst.Close() //nolint:errcheck // test proxy
	defer src.Close() //nolint:errcheck // test proxy
	buf := make([]byte, 32<<10)
	for {
		if p.idle > 0 {
			_ = src.SetReadDeadline(time.Now().Add(p.idle))
		}
		n, err := src.Read(buf)
		if err != nil {
			return
		}
		if p.frozen.Load() {
			continue
		}
		if _, err := dst.Write(buf[:n]); err != nil {
			return
		}
	}
}

// readOne reads one byte from c in the background; the channel gets the error.
func readOne(c net.Conn) <-chan error {
	done := make(chan error, 1)
	go func() {
		_, err := io.ReadFull(c, make([]byte, 1))
		done <- err
	}()
	return done
}

// A balancer that closes idle connections after 120ms keeps one that is pinged
// every 50ms; without pings (the control) it closes it.
func TestPingKeepsIdleConnection(t *testing.T) {
	for _, c := range []struct {
		name  string
		ping  time.Duration
		alive bool
	}{{"pinged", 50 * time.Millisecond, true}, {"not pinged", time.Hour, false}} {
		t.Run(c.name, func(t *testing.T) {
			withPing(t, c.ping)
			ln := listen(t)
			px := newProxy(t, ln.Addr().String(), 120*time.Millisecond)
			client := dial(t, wsURL(px.ln.Addr().String()), nil)
			server := accept(t, ln)
			go func() { _, _ = io.Copy(io.Discard, client) }() // answers pings
			read := readOne(server)                            // reads pongs

			time.Sleep(500 * time.Millisecond) // idle: no SSH bytes
			_, _ = client.Write([]byte{1})
			select {
			case err := <-read:
				if c.alive {
					require.NoError(t, err, "the idle connection is alive")
				} else {
					require.Error(t, err, "the balancer closed the idle connection")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("no byte and no error")
			}
		})
	}
}

// A peer that stops answering is closed within two ping intervals.
func TestPeerGoneClosesConn(t *testing.T) {
	const interval = 50 * time.Millisecond
	withPing(t, interval)
	ln := listen(t)
	px := newProxy(t, ln.Addr().String(), 0)
	client := dial(t, wsURL(px.ln.Addr().String()), nil)
	server := accept(t, ln)
	go func() { _, _ = io.Copy(io.Discard, client) }()
	read := readOne(server)

	start := time.Now()
	px.frozen.Store(true)
	select {
	case err := <-read:
		require.Error(t, err)
		require.LessOrEqual(t, time.Since(start), 2*interval+250*time.Millisecond)
	case <-time.After(2 * time.Second):
		t.Fatal("the connection of a silent peer is still open")
	}
}

// A keep-alive connection that got an answer other than the upgrade is
// closed after idleTimeout without a next request.
func TestIdleKeepAliveClosed(t *testing.T) {
	old := idleTimeout
	idleTimeout = 100 * time.Millisecond
	t.Cleanup(func() { idleTimeout = old })
	c, err := net.Dial("tcp", listen(t).Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	_, err = io.WriteString(c, "GET / HTTP/1.1\r\nHost: gw\r\n\r\n")
	require.NoError(t, err)
	br := bufio.NewReader(c)
	resp, err := http.ReadResponse(br, nil)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.NoError(t, c.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, err = br.ReadByte()
	require.ErrorIs(t, err, io.EOF, "the server closes the idle connection")
}
