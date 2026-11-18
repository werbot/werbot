//go:build load

package load

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"google.golang.org/protobuf/proto"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/gen/gateway/v1/gatewayv1connect"
	"github.com/werbot/werbot/test/cluster"
)

const simHeartbeat = 15 * time.Second // the gateway's heartbeat interval

// snap is one Snapshot a sim received: its version, when it arrived and its proto.Size.
type snap struct {
	version string
	at      time.Time
	size    int
}

// sim is a simulated gateway instance: only the tunnel stream, no SSH.
// run sets start before the first snapshot and err before it closes got.
type sim struct {
	name  string
	got   chan snap
	start time.Time // the stream was opened
	err   error     // why the stream ended
	tr    *http.Transport
}

// enrollSim enrolls instance "sim-<i>" with token through Edge and returns it
// with its tunnel client: HTTP/2 only, TLS 1.3, the enrollment CA and client
// certificate, ServerName from tunnel_url, every connection dialed to tunnelAddr
// (the published nginx :8443).
func enrollSim(ctx context.Context, st *cluster.Stand, token, tunnelAddr string, i int) (*sim, gatewayv1connect.TunnelServiceClient, error) {
	name := fmt.Sprintf("sim-%d", i)
	res, key, err := cluster.Enroll(ctx, st.Edge, token, name, "load")
	if err != nil {
		return nil, nil, fmt.Errorf("%s: enroll: %w", name, err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	cert, err := tls.X509KeyPair([]byte(res.GetCertPem()), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	if err != nil {
		return nil, nil, fmt.Errorf("%s: client certificate: %w", name, err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(res.GetCaPem())) {
		return nil, nil, fmt.Errorf("%s: no CA certificate in ca_pem", name)
	}
	u, err := url.Parse(res.GetTunnelUrl())
	if err != nil {
		return nil, nil, fmt.Errorf("%s: tunnel_url: %w", name, err)
	}
	var protos http.Protocols
	protos.SetHTTP2(true)
	var d net.Dialer
	tr := &http.Transport{
		Protocols: &protos,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS13, RootCAs: roots, ServerName: u.Hostname(),
			NextProtos: []string{"h2"}, Certificates: []tls.Certificate{cert},
		},
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return d.DialContext(ctx, network, tunnelAddr)
		},
	}
	return &sim{name: name, got: make(chan snap, 16), tr: tr},
		gatewayv1connect.NewTunnelServiceClient(&http.Client{Transport: tr}, res.GetTunnelUrl()), nil
}

// run opens the Connect stream, sends a heartbeat at once and every
// simHeartbeat, and passes every Snapshot to got until the stream or ctx ends.
func (s *sim) run(ctx context.Context, c gatewayv1connect.TunnelServiceClient) {
	defer close(s.got)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s.start = time.Now()
	stream := c.Connect(ctx)
	defer func() { _ = stream.CloseResponse() }()
	// as the gateway does: ending the body lets the HTTP/2 transport abort the stream
	context.AfterFunc(ctx, func() { _ = stream.CloseRequest() })
	beat := &gatewayv1.ConnectRequest{Msg: &gatewayv1.ConnectRequest_Heartbeat{Heartbeat: &gatewayv1.Heartbeat{Version: "load", Connectors: []string{"ssh"}}}}
	if err := stream.Send(beat); err != nil && !errors.Is(err, io.EOF) { // on EOF Receive returns why
		s.err = err
		return
	}
	go func() {
		tick := time.NewTicker(simHeartbeat)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if stream.Send(beat) != nil {
					cancel()
					return
				}
			}
		}
	}()
	for {
		msg, err := stream.Receive()
		if err != nil {
			s.err = err
			return
		}
		if x := msg.GetSnapshot(); x != nil {
			select {
			case s.got <- snap{version: x.GetVersion(), at: time.Now(), size: proto.Size(x)}:
			case <-ctx.Done():
				s.err = ctx.Err()
				return
			}
		}
	}
}

// next returns the first snapshot in got whose version differs from prev and
// that arrived by deadline.
func (s *sim) next(prev string, deadline time.Time) (snap, error) {
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	for {
		select {
		case x, ok := <-s.got:
			switch {
			case !ok:
				return snap{}, fmt.Errorf("%s: stream ended: %w", s.name, s.err)
			case x.version == prev:
				continue
			case x.at.After(deadline):
				return snap{}, fmt.Errorf("%s: new version after the deadline", s.name)
			}
			return x, nil
		case <-timer.C:
			return snap{}, fmt.Errorf("%s: no new version by the deadline", s.name)
		}
	}
}

// drain empties ch and returns the version of its last snapshot, or v when none.
func drain(ch <-chan snap, v string) string {
	for {
		select {
		case x, ok := <-ch:
			if !ok {
				return v
			}
			v = x.version
		default:
			return v
		}
	}
}
