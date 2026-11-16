package gateway

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/gen/gateway/v1/gatewayv1connect"
	"github.com/werbot/werbot/internal/platform/metrics"
)

// renewRetry is the pause after a failed certificate renewal.
const renewRetry = time.Minute

// errRenewed cancels the stream that still runs on the replaced certificate.
var errRenewed = errors.New("client certificate renewed")

// tunnelConn is one transport with its client; renewal swaps both.
type tunnelConn struct {
	tr  *http.Transport
	rpc gatewayv1connect.TunnelServiceClient
}

// tunnelClient keeps the Connect stream to the control plane and makes its unary calls.
type tunnelClient struct {
	heartbeatEvery, backoffMin, backoffMax, callTimeout time.Duration

	st        state
	roots     *x509.CertPool
	certs     *certStore
	log       *slog.Logger
	gauge     *metrics.Gauge
	conn      atomic.Pointer[tunnelConn]
	policy    atomic.Pointer[Policy]
	connected atomic.Bool

	mu           sync.Mutex
	cancelStream context.CancelCauseFunc // the current stream
}

func newTunnelClient(st state, roots *x509.CertPool, certs *certStore, reg *metrics.Registry, logger *slog.Logger) *tunnelClient {
	c := &tunnelClient{
		heartbeatEvery: 15 * time.Second, backoffMin: time.Second, backoffMax: 30 * time.Second, callTimeout: 10 * time.Second,
		st: st, roots: roots, certs: certs, log: logger,
		gauge: reg.Gauge("werbot_gateway_tunnel_connected", "1 while the tunnel stream is open and has delivered a snapshot."),
	}
	c.dial()
	return c
}

// newTransport is the HTTP/2-only mTLS transport of the tunnel: TLS 1.3, the pinned CA
// only, the live client certificate and HTTP/2 pings that detect a dead connection.
func newTransport(roots *x509.CertPool, certs *certStore, serverName string) *http.Transport {
	var protos http.Protocols
	protos.SetHTTP2(true)
	return &http.Transport{
		Protocols: &protos,
		TLSClientConfig: &tls.Config{
			MinVersion:           tls.VersionTLS13,
			RootCAs:              roots,
			ServerName:           serverName,
			NextProtos:           []string{"h2"},
			GetClientCertificate: certs.GetClientCertificate,
		},
		HTTP2:           &http.HTTP2Config{SendPingTimeout: 15 * time.Second, PingTimeout: 10 * time.Second},
		IdleConnTimeout: 90 * time.Second, // closes the connection left behind by a renewal
	}
}

// dial swaps in a fresh transport, so the next connection presents the current certificate.
func (c *tunnelClient) dial() {
	host := "" // an invalid URL fails every call
	if u, err := url.Parse(c.st.TunnelURL); err == nil {
		host = u.Hostname()
	}
	tr := newTransport(c.roots, c.certs, host)
	old := c.conn.Swap(&tunnelConn{tr: tr, rpc: gatewayv1connect.NewTunnelServiceClient(&http.Client{Transport: tr}, c.st.TunnelURL)})
	if old != nil {
		old.tr.CloseIdleConnections()
	}
}

// Policy is the latest snapshot; nil until the first one arrives.
func (c *tunnelClient) Policy() *Policy { return c.policy.Load() }

// Connected reports whether the stream is open and has delivered a snapshot.
func (c *tunnelClient) Connected() bool { return c.connected.Load() }

func (c *tunnelClient) setConnected(v bool) {
	c.connected.Store(v)
	if v {
		c.gauge.Set(1)
	} else {
		c.gauge.Set(0)
	}
}

// Run keeps the stream open and the certificate fresh until ctx ends, then returns nil.
// A broken stream leaves active sessions and the last snapshot alone.
func (c *tunnelClient) Run(ctx context.Context, heartbeat func() *gatewayv1.Heartbeat) error {
	var wg sync.WaitGroup
	wg.Go(func() { c.renewLoop(ctx) })
	defer wg.Wait()
	defer func() { c.conn.Load().tr.CloseIdleConnections() }()
	failures := 0
	for {
		got, err := c.stream(ctx, heartbeat)
		if ctx.Err() != nil {
			return nil
		}
		if got {
			failures = 0
		}
		if retryNow(err, got) {
			c.log.InfoContext(ctx, "tunnel: reconnecting", "reason", err)
			continue
		}
		d := c.backoff(failures)
		failures++
		lvl := slog.LevelWarn
		if connect.CodeOf(err) == connect.CodeUnauthenticated { // revoked instance or unknown serial
			lvl = slog.LevelError
		}
		c.log.Log(ctx, lvl, "tunnel: stream failed", "error", err, "retry_in", d)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(d):
		}
	}
}

// renewLoop renews the certificate at 2/3 of its lifetime; a failure retries after renewRetry.
func (c *tunnelClient) renewLoop(ctx context.Context) {
	t := time.NewTimer(time.Until(renewAt(c.certs.Leaf())))
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		next := renewRetry
		if err := c.renew(ctx); err != nil {
			c.log.ErrorContext(ctx, "tunnel: certificate renewal failed", "error", err, "retry_in", next)
		} else {
			next = time.Until(renewAt(c.certs.Leaf()))
		}
		t.Reset(next)
	}
}

// renew gets a certificate for a new key, stores the pair and reconnects with it.
// The key rotates, so the crash window of certStore.Replace applies: a crash between
// its two renames leaves an unloadable pair and the instance must re-enroll.
func (c *tunnelClient) renew(ctx context.Context) error {
	keyPEM, csr, err := newCSR()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	resp, err := c.conn.Load().rpc.RenewCertificate(ctx, connect.NewRequest(&gatewayv1.RenewCertificateRequest{CsrDer: csr}))
	if err != nil {
		return fmt.Errorf("renew certificate: %w", err)
	}
	if err := verifyClientCert(resp.Msg.GetCertPem(), c.roots); err != nil {
		return fmt.Errorf("renew certificate: %w", err)
	}
	if err := c.certs.Replace(resp.Msg.GetCertPem(), keyPEM); err != nil {
		return err
	}
	c.dial()
	c.mu.Lock()
	if c.cancelStream != nil {
		c.cancelStream(errRenewed)
	}
	c.mu.Unlock()
	return nil
}
