package tcpconnector_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	_ "github.com/werbot/werbot/internal/connector/tcp" // the tcp resource type
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/connectortest"
	"github.com/werbot/werbot/internal/gateway/tcpconnector"
)

const waitFor = 5 * time.Second

// listen starts a TCP target; handle serves each accepted connection.
func listen(t *testing.T, handle func(net.Conn)) (addr string, accepts func() int) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	l, accepts = connectortest.CountAccepts(l)
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); handle(c) }()
		}
	}()
	return l.Addr().String(), accepts
}

func resource(addr string) gateway.Resource {
	host, port, _ := net.SplitHostPort(addr)
	cfg := fmt.Sprintf(`{"host":%q,"ports":[{"port":%s}]}`, host, port)
	return gateway.Resource{ID: uuid.New(), Name: "svc", Type: "tcp", Config: []byte(cfg)}
}

func echo(c net.Conn) { _, _ = io.Copy(c, c) }

func TestContract(t *testing.T) {
	registered, ok := gateway.LookupConnector("tcp")
	require.True(t, ok)
	require.Equal(t, tcpconnector.Connector{}, registered)
	addr, accepts := listen(t, echo)
	connectortest.Run(t, tcpconnector.Connector{}, connectortest.Target{
		Resource: resource(addr), Secret: gateway.Secret{Kind: "none", Value: []byte("werbot-tcp-secret")},
		Stream: addr, Accepts: accepts,
	})
}

func TestDialsTarget(t *testing.T) {
	var c gateway.Connector = tcpconnector.Connector{}
	_, ok := c.(gateway.TargetDialer)
	require.True(t, ok)
}

// session returns a Session whose Stream is one end of a loopback TCP pair (it has CloseWrite)
// and the client end.
func session(t *testing.T, addr string) (*gateway.Session, *net.TCPConn) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	cc, err := net.Dial("tcp", l.Addr().String())
	require.NoError(t, err)
	sc, err := l.Accept()
	require.NoError(t, err)
	t.Cleanup(func() { _ = cc.Close(); _ = sc.Close() })
	d := &net.Dialer{Timeout: waitFor}
	return &gateway.Session{
		ID: uuid.New(), Kind: gateway.KindStream, Resource: resource(addr), Stream: sc, Target: addr,
		Secret: gateway.Secret{Kind: "none", Value: []byte("tcp-secret-value")},
		Dial:   d.DialContext,
	}, cc.(*net.TCPConn)
}

// The client half-closes after "ping"; the target answers "pong" only after it sees EOF.
func TestTCPHalfClose(t *testing.T) {
	addr, _ := listen(t, func(c net.Conn) {
		b, _ := io.ReadAll(c)
		if string(b) == "ping" {
			_, _ = io.WriteString(c, "pong")
		}
	})
	s, client := session(t, addr)
	served := make(chan error, 1)
	go func() { served <- tcpconnector.Connector{}.Serve(t.Context(), s) }()

	_, err := io.WriteString(client, "ping")
	require.NoError(t, err)
	require.NoError(t, client.CloseWrite())
	require.NoError(t, client.SetReadDeadline(time.Now().Add(waitFor)))
	got, err := io.ReadAll(client)
	require.NoError(t, err)
	require.Equal(t, "pong", string(got))
	select {
	case err := <-served:
		require.NoError(t, err)
	case <-time.After(waitFor):
		t.Fatal("Serve did not return after both sides closed")
	}
}

func TestTCPDialFailure(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close()) // a closed port
	s, _ := session(t, addr)

	start := time.Now()
	err = tcpconnector.Connector{}.Serve(t.Context(), s)
	require.Error(t, err)
	require.Less(t, time.Since(start), 10*time.Second)
	require.NotContains(t, err.Error(), addr)
	require.NotContains(t, err.Error(), "127.0.0.1")
	require.NotContains(t, err.Error(), string(s.Secret.Value))
}

func TestTCPNoGoroutineLeak(t *testing.T) {
	addr, _ := listen(t, echo)
	base := runtime.NumGoroutine()
	for range 100 {
		s, client := session(t, addr)
		served := make(chan error, 1)
		go func() { served <- tcpconnector.Connector{}.Serve(t.Context(), s) }()
		_, err := io.WriteString(client, "x")
		require.NoError(t, err)
		b := make([]byte, 1)
		_, err = io.ReadFull(client, b)
		require.NoError(t, err)
		require.NoError(t, client.Close())
		require.NoError(t, <-served)
	}
	require.Eventually(t, func() bool { return runtime.NumGoroutine() <= base+2 }, waitFor, 10*time.Millisecond,
		"goroutines: %d, base %d", runtime.NumGoroutine(), base)
}

// A cancel closes both sides and is not an error.
func TestTCPCancel(t *testing.T) {
	addr, _ := listen(t, echo)
	s, client := session(t, addr)
	ctx, cancel := context.WithCancel(t.Context())
	served := make(chan error, 1)
	go func() { served <- tcpconnector.Connector{}.Serve(ctx, s) }()
	_, err := io.WriteString(client, "x")
	require.NoError(t, err)
	_, err = io.ReadFull(client, make([]byte, 1))
	require.NoError(t, err)
	cancel()
	require.NoError(t, <-served)
	require.NoError(t, client.SetReadDeadline(time.Now().Add(waitFor)))
	_, err = client.Read(make([]byte, 1))
	require.ErrorIs(t, err, io.EOF)
}

// A relay panic ends only that session and re-panics on the Serve caller.
func TestTCPRelayPanic(t *testing.T) {
	addr, _ := listen(t, echo)
	s, _ := session(t, addr)
	s.Stream = panicStream{s.Stream}
	var p any
	func() {
		defer func() { p = recover() }()
		_ = tcpconnector.Connector{}.Serve(t.Context(), s)
	}()
	require.Contains(t, fmt.Sprint(p), "fault injected")
	require.False(t, strings.Contains(fmt.Sprint(p), "tcp-secret-value"))
}

type panicStream struct{ io.ReadWriteCloser }

func (panicStream) Read([]byte) (int, error) { panic("fault injected") }
