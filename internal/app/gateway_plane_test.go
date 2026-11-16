package app

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"regexp"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/gen/gateway/v1/gatewayv1connect"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/tlsca"
)

func TestBootstrapCreatesCAOnlyForServer(t *testing.T) {
	pool, url := dbtest.New(t)
	getenv := env(map[string]string{"WERBOT_DATABASE_URL": url, "WERBOT_PUBLIC_URL": testPublicURL, "WERBOT_SECRET_KEY": testSecretKey,
		"WERBOT_HTTP_ADDR": freeAddr(t), "WERBOT_GATEWAY_ADDR": freeAddr(t)})
	caKeys := func() int {
		var n int
		require.NoError(t, pool.QueryRow(context.Background(), "SELECT count(*) FROM ca_key").Scan(&n))
		return n
	}

	require.NoError(t, Run(context.Background(), []string{"migrate"}, getenv, io.Discard))
	assert.Equal(t, 0, caKeys())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, []string{"server"}, getenv, io.Discard) }()
	require.Eventually(t, func() bool {
		res, err := http.Get("http://" + getenv("WERBOT_HTTP_ADDR") + "/readyz")
		if err != nil {
			return false
		}
		_ = res.Body.Close()
		return res.StatusCode == http.StatusOK
	}, 10*time.Second, 20*time.Millisecond)
	assert.Equal(t, 1, caKeys())
	cancel()
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("Run did not return")
	}
}

func TestServeGatewayAddrInUse(t *testing.T) {
	pool, _ := dbtest.New(t)
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = busy.Close() }()
	cfg := config.Config{HTTPAddr: freeAddr(t), GatewayAddr: busy.Addr().String(), WebEmbedded: true}
	withTestDefaults(t, &cfg)
	o := testServeOpts(t, cfg, pool)
	done := make(chan error, 1)
	go func() {
		done <- serve(context.Background(), cfg, slog.New(slog.DiscardHandler), pool, nil, fstest.MapFS{}, o)
	}()
	select {
	case err := <-done:
		assert.ErrorContains(t, err, cfg.GatewayAddr)
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return on a busy gateway address")
	}
	_, err = net.Dial("tcp", cfg.HTTPAddr)
	assert.Error(t, err, "the HTTP listener is closed again")
}

func TestGeoIPReloaderSingleFlight(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var running, peak, calls atomic.Int32
	release := make(chan struct{})
	kick := geoIPReloader(ctx, func(context.Context) error {
		calls.Add(1)
		if n := running.Add(1); n > peak.Load() {
			peak.Store(n)
		}
		<-release
		running.Add(-1)
		return errors.New("broken")
	}, slog.New(slog.DiscardHandler))

	kick("")
	require.Eventually(t, func() bool { return calls.Load() == 1 }, 5*time.Second, time.Millisecond)
	for range 10 {
		kick("") // never blocks while a reload runs
	}
	close(release)
	require.Eventually(t, func() bool { return calls.Load() == 2 && running.Load() == 0 }, 5*time.Second, time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(2), calls.Load(), "triggers during a reload give one more pass")
	assert.Equal(t, int32(1), peak.Load())
}

// enrolled is a gateway instance: its key, certificate and tunnel client.
type enrolled struct {
	key            ed25519.PrivateKey
	res            *werbotv1.EnrollResponse
	client         gatewayv1connect.TunnelServiceClient
	closeTransport func()
}

func newCSR(t *testing.T) (ed25519.PrivateKey, []byte) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, key)
	require.NoError(t, err)
	return key, csr
}

// tunnelClient dials url over HTTP/2 with cert, trusting only caPEM.
func tunnelClient(t *testing.T, url string, caPEM []byte, cert tls.Certificate) (gatewayv1connect.TunnelServiceClient, func()) {
	t.Helper()
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(caPEM))
	var protos http.Protocols
	protos.SetHTTP2(true)
	tr := &http.Transport{Protocols: &protos, TLSClientConfig: &tls.Config{RootCAs: roots, Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13}}
	t.Cleanup(tr.CloseIdleConnections)
	return gatewayv1connect.NewTunnelServiceClient(&http.Client{Transport: tr}, url), tr.CloseIdleConnections
}

func withCert(t *testing.T, key ed25519.PrivateKey, certPEM []byte) tls.Certificate {
	t.Helper()
	blk, _ := pem.Decode(certPEM)
	require.NotNil(t, blk)
	return tls.Certificate{Certificate: [][]byte{blk.Bytes}, PrivateKey: key}
}

func enroll(t *testing.T, enrollSvc werbotv1connect.GatewayEnrollServiceClient, token string) enrolled {
	t.Helper()
	key, csr := newCSR(t)
	res, err := enrollSvc.Enroll(context.Background(), connect.NewRequest(&werbotv1.EnrollRequest{
		JoinToken: token, CsrDer: csr, Hostname: "gw", Version: "test"}))
	require.NoError(t, err)
	e := enrolled{key: key, res: res.Msg}
	e.client, e.closeTransport = tunnelClient(t, res.Msg.GetTunnelUrl(), []byte(res.Msg.GetCaPem()), withCert(t, key, []byte(res.Msg.GetCertPem())))
	return e
}

// openStream sends a heartbeat and returns the first snapshot and a channel
// with the stream's final error.
func openStream(t *testing.T, c gatewayv1connect.TunnelServiceClient) (*gatewayv1.Snapshot, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	st := c.Connect(ctx)
	require.NoError(t, st.Send(&gatewayv1.ConnectRequest{Msg: &gatewayv1.ConnectRequest_Heartbeat{Heartbeat: &gatewayv1.Heartbeat{Version: "test"}}}))
	first, err := st.Receive()
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		for {
			if _, err := st.Receive(); err != nil {
				done <- err
				return
			}
		}
	}()
	return first.GetSnapshot(), done
}

func streamEnd(t *testing.T, done <-chan error, within time.Duration) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(within):
		t.Fatalf("stream still open after %v", within)
		return nil
	}
}

// foreignCert is a self-signed client certificate with a valid gateway CN.
func foreignCert(t *testing.T) tls.Certificate {
	t.Helper()
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: tlsca.GatewayCN(uuid.New(), uuid.New())},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, pub, key)
	require.NoError(t, err)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func TestServeGatewayPlane(t *testing.T) {
	pool, _ := dbtest.New(t)
	httpAddr := freeAddr(t)
	cfg := config.Config{HTTPAddr: httpAddr, MetricsAddr: freeAddr(t), GatewayAddr: freeAddr(t), Env: "development",
		PublicURL: "http://" + httpAddr, WebEmbedded: true}
	stop := startServe(t, cfg, pool, nil, fstest.MapFS{}, slog.New(slog.DiscardHandler))
	ctx, api := context.Background(), "http://"+httpAddr

	hc, _ := jarClient(t, http.DefaultTransport)
	_, err := werbotv1connect.NewAuthServiceClient(hc, api).Setup(ctx, connect.NewRequest(ownerSetup))
	require.NoError(t, err)
	gws := werbotv1connect.NewGatewayServiceClient(hc, api)
	created, err := gws.CreateGateway(ctx, connect.NewRequest(&werbotv1.CreateGatewayRequest{Name: "edge"}))
	require.NoError(t, err)
	tok, err := gws.CreateJoinToken(ctx, connect.NewRequest(&werbotv1.CreateJoinTokenRequest{GatewayId: created.Msg.GetGateway().GetId()}))
	require.NoError(t, err)
	enrollSvc := werbotv1connect.NewGatewayEnrollServiceClient(http.DefaultClient, api)

	gw := enroll(t, enrollSvc, tok.Msg.GetToken())
	_, port, err := net.SplitHostPort(cfg.GatewayAddr)
	require.NoError(t, err)
	assert.Equal(t, "https://127.0.0.1:"+port, gw.res.GetTunnelUrl())

	snap, done := openStream(t, gw.client)
	assert.NotEmpty(t, snap.GetVersion())
	assert.Empty(t, snap.GetResources())
	assert.Eventually(t, func() bool {
		_, body := get(t, "http://"+cfg.MetricsAddr+"/metrics")
		return regexp.MustCompile(`(?m)^werbot_tunnel_streams 1$`).MatchString(body)
	}, 5*time.Second, 20*time.Millisecond)

	// Renewal: the old serial stops authenticating at once.
	key, csr := newCSR(t)
	renewed, err := gw.client.RenewCertificate(ctx, connect.NewRequest(&gatewayv1.RenewCertificateRequest{CsrDer: csr}))
	require.NoError(t, err)
	auth := func(c gatewayv1connect.TunnelServiceClient) error {
		_, err := c.AuthenticateUser(ctx, connect.NewRequest(&gatewayv1.AuthenticateUserRequest{
			By: &gatewayv1.AuthenticateUserRequest_Login{Login: "nobody"}, ClientIp: "10.0.0.1"}))
		return err
	}
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(auth(gw.client)), "old certificate")
	fresh, _ := tunnelClient(t, gw.res.GetTunnelUrl(), []byte(gw.res.GetCaPem()), withCert(t, key, renewed.Msg.GetCertPem()))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(auth(fresh)), "new certificate")
	gw.closeTransport()
	assert.Error(t, streamEnd(t, done, 5*time.Second))

	// A certificate of another CA fails the TLS handshake: no server answer.
	other, _ := tunnelClient(t, gw.res.GetTunnelUrl(), []byte(gw.res.GetCaPem()), foreignCert(t))
	err = auth(other)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tls:")

	// Deleting the instance closes its stream: trigger -> NOTIFY -> Hub.
	_, done = openStream(t, fresh)
	_, err = gws.DeleteGatewayInstance(ctx, connect.NewRequest(&werbotv1.DeleteGatewayInstanceRequest{Id: gw.res.GetInstanceId()}))
	require.NoError(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(streamEnd(t, done, 5*time.Second)))

	// Shutdown closes the streams with Unavailable "shutdown" (hub.Shutdown,
	// not a dropped connection) before the tunnel server stops.
	second := enroll(t, enrollSvc, tok.Msg.GetToken())
	_, done = openStream(t, second.client)
	stop()
	var ce *connect.Error
	require.ErrorAs(t, streamEnd(t, done, time.Second), &ce)
	assert.Equal(t, connect.CodeUnavailable, ce.Code())
	assert.Equal(t, "shutdown", ce.Message())
}
