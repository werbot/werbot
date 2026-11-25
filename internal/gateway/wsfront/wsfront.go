// Package wsfront is the gateway's WebSocket input: SSH carried in binary
// WebSocket messages on GET /v1/ssh, so clients reach the gateway through one
// HTTPS port behind an L7 load balancer that terminates TLS.
package wsfront

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/werbot/werbot/internal/platform/clientip"
)

// Path is the only path that upgrades.
const Path = "/v1/ssh"

// pingInterval is how often an accepted connection is pinged; a peer that does
// not answer within the same interval is closed. Tests shorten it.
var pingInterval = 30 * time.Second

// idleTimeout closes a keep-alive connection that sends no next request.
// Tests shorten it.
var idleTimeout = 60 * time.Second

// Listener listens on addr and returns the connections upgraded on GET Path as
// a net.Listener. With tlsCert and tlsKey (PEM files, both or neither) it
// serves TLS 1.2 or later. X-Forwarded-For counts only from a peer inside
// trusted (clientip). Close stops new upgrades; accepted connections live on.
func Listener(addr, tlsCert, tlsKey string, trusted []netip.Prefix) (net.Listener, error) {
	var tc *tls.Config
	if tlsCert != "" || tlsKey != "" {
		cert, err := tls.LoadX509KeyPair(tlsCert, tlsKey)
		if err != nil {
			return nil, fmt.Errorf("websocket tls: %w", err)
		}
		tc = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	}
	tcp, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	l := &listener{tcp: tcp, conns: make(chan net.Conn), done: make(chan struct{}), ping: pingInterval}
	mux := http.NewServeMux()
	mux.Handle("GET "+Path, clientip.Middleware(trusted)(http.HandlerFunc(l.upgrade)))
	l.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: idleTimeout}
	ln := tcp
	if tc != nil {
		ln = tls.NewListener(tcp, tc) // HTTP/1.1 only: no h2 in NextProtos
	}
	go func() { l.stop(l.srv.Serve(ln)) }()
	return l, nil
}

type listener struct {
	tcp   net.Listener
	srv   *http.Server
	conns chan net.Conn
	ping  time.Duration

	once sync.Once
	done chan struct{} // closed with err set once the listener stops
	err  error
}

// stop ends Accept with err; http.ErrServerClosed (Close) is net.ErrClosed.
func (l *listener) stop(err error) {
	l.once.Do(func() {
		if errors.Is(err, http.ErrServerClosed) {
			err = net.ErrClosed
		}
		l.err = err
		close(l.done)
	})
}

func (l *listener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.done:
		return nil, l.err
	}
}

// Close stops the HTTP server and the TCP listener; hijacked connections are not its.
func (l *listener) Close() error {
	err := l.srv.Close()
	l.stop(net.ErrClosed)
	return err
}

func (l *listener) Addr() net.Addr { return l.tcp.Addr() }

// upgrade accepts the WebSocket (default Origin check, no compression), hands
// the connection to Accept and pings it until it closes.
func (l *listener) upgrade(w http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Accept(w, r, nil)
	if err != nil {
		return // Accept answered the request
	}
	ctx, cancel := context.WithCancel(r.Context()) // r.Context lives while this handler runs
	defer cancel()
	c := &conn{Conn: websocket.NetConn(ctx, ws, websocket.MessageBinary), remote: remoteAddr(r), cancel: cancel}
	select {
	case l.conns <- c:
	case <-l.done:
		_ = ws.CloseNow()
		return
	}
	keepAlive(ctx, ws, l.ping)
	_ = ws.CloseNow()
}

// keepAlive pings ws every d until ctx ends or a pong takes longer than d.
// Pongs are read by the connection's reader (the SSH server always reads).
func keepAlive(ctx context.Context, ws *websocket.Conn, d time.Duration) {
	t := time.NewTicker(d)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		pctx, cancel := context.WithTimeout(ctx, d)
		err := ws.Ping(pctx)
		cancel()
		if err != nil {
			return
		}
	}
}

// remoteAddr is the client of r: the IP clientip resolved, with the TCP peer's
// port when that IP is the peer's and port 0 for an X-Forwarded-For address.
func remoteAddr(r *http.Request) *net.TCPAddr {
	ip := clientip.From(r.Context())
	peer, _ := netip.ParseAddrPort(r.RemoteAddr)
	var port uint16
	if peer.Addr().Unmap() == ip {
		port = peer.Port()
	}
	return net.TCPAddrFromAddrPort(netip.AddrPortFrom(ip, port))
}

// conn is an upgraded connection: SSH bytes in binary messages; a text
// message closes it.
type conn struct {
	net.Conn
	remote net.Addr
	cancel context.CancelFunc // ends keepAlive
}

func (c *conn) RemoteAddr() net.Addr { return c.remote }

// Transport names the input for the session record.
func (*conn) Transport() string { return "ws" }

func (c *conn) Close() error {
	defer c.cancel()
	return c.Conn.Close()
}
