package relay

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/tlsca"
)

// client dials url over TLS 1.3 trusting ca, presenting cert when it is not
// nil; http2 selects the protocol (HTTP/1.1 otherwise).
func client(ca *tlsca.CA, cert func(*tls.CertificateRequestInfo) (*tls.Certificate, error), http2 bool) *http.Client {
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(ca.RootPEM())
	var protos http.Protocols
	protos.SetHTTP1(!http2)
	protos.SetHTTP2(http2)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS13, GetClientCertificate: cert},
		Protocols:       &protos,
	}
	return &http.Client{Transport: tr, Timeout: 5 * time.Second}
}

// gatewayCert is a client certificate of ca with a gateway common name.
func gatewayCert(t *testing.T, ca *tlsca.CA) func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, key)
	require.NoError(t, err)
	certPEM, err := ca.IssueClient(csr, tlsca.GatewayCN(uuid.New(), uuid.New()), time.Hour)
	require.NoError(t, err)
	blk, _ := pem.Decode(certPEM)
	return func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		return &tls.Certificate{Certificate: [][]byte{blk.Bytes}, PrivateKey: key}, nil
	}
}

func TestInternalListenerAcceptsOnlyKnownServers(t *testing.T) {
	pool, _ := dbtest.New(t)
	ca := newCA(t)
	r := newRelay(t, pool, ca)
	r.mux.HandleFunc("/ping", func(w http.ResponseWriter, req *http.Request) {
		if req.ProtoMajor != 2 {
			http.Error(w, "want HTTP/2", http.StatusTeapot)
			return
		}
		_, _ = w.Write([]byte("pong"))
	})
	start(t, r)
	url := r.cfg.InternalURL + "/ping"

	get := func(c *http.Client) (int, error) {
		resp, err := c.Get(url)
		if err != nil {
			return 0, err
		}
		_ = resp.Body.Close()
		return resp.StatusCode, nil
	}

	code, err := get(client(ca, r.clientCert, true))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, code, "registered replica over HTTP/2")

	unknown := New(pool, ca, nil, Config{InstanceID: uuid.New()}, r.log)
	code, err = get(client(ca, unknown.clientCert, true))
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, code, "replica without a server_instance row")

	code, err = get(client(ca, gatewayCert(t, ca), true))
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, code, "gateway certificate of the same CA")

	_, err = get(client(ca, nil, true))
	assert.Error(t, err, "no client certificate fails the TLS handshake")

	other := New(pool, newCA(t), nil, Config{InstanceID: r.cfg.InstanceID}, r.log)
	_, err = get(client(ca, other.clientCert, true))
	assert.Error(t, err, "certificate of another CA fails the TLS handshake")
}

func TestInternalListenerRefusesHTTP1(t *testing.T) {
	pool, _ := dbtest.New(t)
	ca := newCA(t)
	r := newRelay(t, pool, ca)
	r.mux.HandleFunc("/ping", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("pong")) })
	start(t, r)
	resp, err := client(ca, r.clientCert, false).Get(r.cfg.InternalURL + "/ping")
	if err == nil {
		_ = resp.Body.Close()
	}
	assert.Error(t, err, "HTTP/1.1 from a registered replica is refused")
}

func TestClientCertPerHandshake(t *testing.T) {
	ca := newCA(t)
	r := New(nil, ca, nil, Config{InstanceID: uuid.New()}, slog.New(slog.DiscardHandler))
	parse := func() *x509.Certificate {
		c, err := r.clientCert(nil)
		require.NoError(t, err)
		require.Len(t, c.Certificate, 1)
		assert.Equal(t, r.key, c.PrivateKey)
		x, err := x509.ParseCertificate(c.Certificate[0])
		require.NoError(t, err)
		return x
	}
	a, b := parse(), parse()
	assert.NotEqual(t, a.SerialNumber, b.SerialNumber, "a new certificate per handshake")
	assert.Equal(t, tlsca.ServerCN(r.cfg.InstanceID), a.Subject.CommonName)
	assert.False(t, a.NotAfter.After(time.Now().Add(time.Hour)), "lives at most 1h")
	assert.Equal(t, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, a.ExtKeyUsage)
}
