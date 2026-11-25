package connectcli

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// Endpoint is where the gateway listens: its SSH address and its WebSocket URL.
type Endpoint struct{ SSHAddress, WSURL string }

// Dialer opens the transport connection to the gateway.
type Dialer func(ctx context.Context, ep Endpoint) (net.Conn, error)

var _ Dialer = Dial

const (
	wsPath        = "/v1/ssh" // the gateway's WebSocket path, used when a URL has none
	wsDialTimeout = 10 * time.Second
)

// Transport settings of Dial; tests replace them.
var (
	sshDialTimeout = 5 * time.Second
	proxyFunc      = http.ProxyFromEnvironment // HTTPS_PROXY, HTTP_PROXY, NO_PROXY
	wsTLS          *tls.Config                 // nil: the system roots
)

// Dial connects to the gateway: over TCP to ep.SSHAddress within
// sshDialTimeout, else (no address or a failure) over WebSocket to ep.WSURL
// within wsDialTimeout. ctx bounds the dial only, not the connection.
// ponytail: the transport is chosen again on every dial; keep the choice for
// the process if 5 s per reconnect to an SSH-blocked gateway gets noticeable.
func Dial(ctx context.Context, ep Endpoint) (net.Conn, error) {
	if ep.SSHAddress == "" && ep.WSURL == "" {
		return nil, errors.New("gateway address is not set")
	}
	var sshErr error
	if ep.SSHAddress != "" {
		d := net.Dialer{Timeout: sshDialTimeout, KeepAlive: 30 * time.Second}
		c, err := d.DialContext(ctx, "tcp", ep.SSHAddress)
		if err == nil || ep.WSURL == "" || ctx.Err() != nil {
			return c, err
		}
		sshErr = err
	}
	c, err := dialWS(ctx, ep.WSURL)
	if err == nil || sshErr == nil {
		return c, err
	}
	return nil, fmt.Errorf("gateway unreachable: ssh %s: %v; websocket %s: %v", ep.SSHAddress, sshErr, ep.WSURL, err)
}

// dialWS opens a WebSocket to rawURL (an empty path is wsPath) through the
// proxy of proxyFunc and returns it as a connection of binary messages.
func dialWS(ctx context.Context, rawURL string) (net.Conn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if u.Path == "" {
		u.Path = wsPath
	}
	tr := &http.Transport{Proxy: proxyFunc, TLSClientConfig: wsTLS}
	defer tr.CloseIdleConnections()
	dctx, cancel := context.WithTimeout(ctx, wsDialTimeout)
	defer cancel()
	c, _, err := websocket.Dial(dctx, u.String(), &websocket.DialOptions{HTTPClient: &http.Client{Transport: tr}}) //nolint:bodyclose // websocket.Dial owns the body
	if err != nil {
		return nil, err
	}
	nc := websocket.NetConn(context.WithoutCancel(ctx), c, websocket.MessageBinary)
	return wsConn{Conn: nc, remote: wsAddr(urlHostPort(u))}, nil
}

// wsConn is a WebSocket connection whose RemoteAddr is the URL's host:port:
// the library gives a dialed connection an address without a port, which
// known_hosts refuses.
type wsConn struct {
	net.Conn
	remote wsAddr
}

func (c wsConn) RemoteAddr() net.Addr { return c.remote }

type wsAddr string

func (wsAddr) Network() string  { return "websocket" }
func (a wsAddr) String() string { return string(a) }

// HostKeyAddr is the address the gateway host key is known under:
// ep.SSHAddress (port 2222 when it has none), else the host:port of ep.WSURL
// (443 for wss, 80 for ws).
func HostKeyAddr(ep Endpoint) string {
	if ep.SSHAddress != "" {
		return withPort(ep.SSHAddress, defaultGatewayPort)
	}
	u, err := url.Parse(ep.WSURL)
	if err != nil {
		return ""
	}
	return urlHostPort(u)
}

// urlHostPort is the host:port of a ws:// or wss:// URL.
func urlHostPort(u *url.URL) string {
	port := u.Port()
	if port == "" {
		port = "80"
		if u.Scheme == "wss" {
			port = "443"
		}
	}
	return net.JoinHostPort(u.Hostname(), port)
}

// withPort is hostport, or host with port when it has none.
func withPort(hostport, port string) string {
	if _, _, err := net.SplitHostPort(hostport); err == nil {
		return hostport
	}
	return net.JoinHostPort(strings.Trim(hostport, "[]"), port)
}
