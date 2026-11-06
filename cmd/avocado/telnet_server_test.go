package main

import (
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/werbot/werbot/internal/core/scheme/services/telnet"
	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
)

func readExactBytes(t *testing.T, c net.Conn, n int) []byte {
	t.Helper()
	buf := make([]byte, n)
	if _, err := io.ReadFull(c, buf); err != nil {
		t.Fatalf("read %d bytes: %v", n, err)
	}
	return buf
}

func newTestTelnetServer(t *testing.T) (*telnet.Conn, net.Conn) {
	t.Helper()

	local, peer := net.Pipe()
	deadline := time.Now().Add(5 * time.Second)
	_ = local.SetDeadline(deadline)
	_ = peer.SetDeadline(deadline)

	type result struct {
		conn *telnet.Conn
		err  error
	}
	ready := make(chan result, 1)
	go func() {
		conn, err := telnet.NewServer(local)
		ready <- result{conn: conn, err: err}
	}()

	// NewServer writes WILL ECHO, WILL SGA, DO NAWS and DO TTYPE.
	_ = readExactBytes(t, peer, 12)

	res := <-ready
	if res.err != nil {
		t.Fatalf("NewServer: %v", res.err)
	}
	t.Cleanup(func() {
		_ = res.conn.Close()
		_ = peer.Close()
	})
	return res.conn, peer
}

func TestReadTokenSuccess(t *testing.T) {
	server, peer := newTestTelnetServer(t)

	type result struct {
		response *tokenmessage.UseTokenSchemeConnect_Response
		err      error
	}
	done := make(chan result, 1)
	go func() {
		response, err := readToken(server, 3, func(token string) (*tokenmessage.UseTokenSchemeConnect_Response, error) {
			if token != "11111111-1111-1111-1111-111111111111" {
				t.Errorf("token = %q", token)
			}
			return &tokenmessage.UseTokenSchemeConnect_Response{ProfileId: "profile"}, nil
		})
		done <- result{response: response, err: err}
	}()

	if got := string(readExactBytes(t, peer, len("Token: "))); got != "Token: " {
		t.Fatalf("prompt = %q", got)
	}
	if _, err := peer.Write([]byte("11111111-1111-1111-1111-111111111111\r\n")); err != nil {
		t.Fatalf("write token: %v", err)
	}

	res := <-done
	if res.err != nil {
		t.Fatalf("readToken() error = %v", res.err)
	}
	if res.response.GetProfileId() != "profile" {
		t.Fatalf("response = %+v", res.response)
	}
}

func TestReadTokenThreeAttempts(t *testing.T) {
	server, peer := newTestTelnetServer(t)

	calls := 0
	done := make(chan error, 1)
	go func() {
		_, err := readToken(server, 3, func(string) (*tokenmessage.UseTokenSchemeConnect_Response, error) {
			calls++
			return nil, errors.New("invalid token")
		})
		done <- err
	}()

	for attempt := 0; attempt < 3; attempt++ {
		if got := string(readExactBytes(t, peer, len("Token: "))); got != "Token: " {
			t.Fatalf("attempt %d prompt = %q", attempt+1, got)
		}
		if _, err := peer.Write([]byte("bad-token\r\n")); err != nil {
			t.Fatalf("write token: %v", err)
		}
		if got := string(readExactBytes(t, peer, len("Invalid token\r\n"))); got != "Invalid token\r\n" {
			t.Fatalf("attempt %d invalid message = %q", attempt+1, got)
		}
	}

	if err := <-done; err == nil {
		t.Fatal("readToken() error = nil, want failure")
	}
	if calls != 3 {
		t.Fatalf("use calls = %d, want 3", calls)
	}
}

func TestReadTokenTrimsCRNUL(t *testing.T) {
	server, peer := newTestTelnetServer(t)

	gotToken := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		_, err := readToken(server, 3, func(token string) (*tokenmessage.UseTokenSchemeConnect_Response, error) {
			gotToken <- token
			return &tokenmessage.UseTokenSchemeConnect_Response{}, nil
		})
		done <- err
	}()

	_ = readExactBytes(t, peer, len("Token: "))
	if _, err := peer.Write([]byte("11111111-1111-1111-1111-111111111111\r\x00")); err != nil {
		t.Fatalf("write token: %v", err)
	}

	if err := <-done; err != nil {
		t.Fatalf("readToken() error = %v", err)
	}
	if token := <-gotToken; token != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("token = %q", token)
	}
}

func TestIdleConnExtendsDeadline(t *testing.T) {
	local, peer := net.Pipe()
	t.Cleanup(func() { _ = peer.Close() })

	idle := &idleConn{Conn: local, timeout: 100 * time.Millisecond}

	go func() {
		for i := 0; i < 3; i++ {
			if _, err := peer.Write([]byte("x")); err != nil {
				return
			}
			time.Sleep(60 * time.Millisecond)
		}
		time.Sleep(250 * time.Millisecond)
		_ = peer.Close()
	}()

	buf := make([]byte, 1)
	for i := 0; i < 3; i++ {
		if _, err := idle.Read(buf); err != nil {
			t.Fatalf("read %d: %v", i+1, err)
		}
	}

	_, err := idle.Read(buf)
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("read after idle pause = %v, want timeout", err)
	}
}

type deadlineTestConn struct {
	net.Conn
	readDeadline time.Time
}

func (c *deadlineTestConn) Read([]byte) (int, error)    { return 0, io.EOF }
func (c *deadlineTestConn) Write(p []byte) (int, error) { return len(p), nil }
func (c *deadlineTestConn) SetReadDeadline(t time.Time) error {
	c.readDeadline = t
	return nil
}

func TestIdleConnWriteExtendsDeadline(t *testing.T) {
	conn := &deadlineTestConn{}
	idle := &idleConn{Conn: conn, timeout: time.Second}

	if _, err := idle.Read(make([]byte, 1)); err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("read: %v", err)
	}
	first := conn.readDeadline
	if first.IsZero() {
		t.Fatal("read did not set a deadline")
	}

	time.Sleep(5 * time.Millisecond)
	if _, err := idle.Write([]byte("x")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !conn.readDeadline.After(first) {
		t.Fatalf("write did not extend read deadline: first=%v after=%v", first, conn.readDeadline)
	}
}

func TestRemoteIP(t *testing.T) {
	tests := []struct {
		name string
		addr net.Addr
		want string
	}{
		{"ipv4 with port", &net.TCPAddr{IP: net.ParseIP("8.8.8.8"), Port: 12345}, "8.8.8.8"},
		{"ipv6 with port", &net.TCPAddr{IP: net.ParseIP("2001:db8::1"), Port: 12345}, "2001:db8::1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := remoteIP(tt.addr); got != tt.want {
				t.Fatalf("remoteIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
