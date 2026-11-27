package dbproxy_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway/dbproxy"
)

// testCert is a test-only certificate and key; an empty dnsName makes it a CA,
// self-signed when parent is nil.
type testCert struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	pem  []byte // certificate followed by key
}

func newCert(t *testing.T, parent *testCert, dnsName string) *testCert {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: "test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	signer, signKey := tmpl, key
	if dnsName == "" {
		tmpl.IsCA, tmpl.BasicConstraintsValid, tmpl.KeyUsage = true, true, x509.KeyUsageCertSign
	} else {
		tmpl.DNSNames = []string{dnsName}
	}
	if parent != nil {
		signer, signKey = parent.cert, parent.key
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, signer, &key.PublicKey, signKey)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	p := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	p = append(p, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})...)
	return &testCert{cert: cert, key: key, pem: p}
}

func (c *testCert) certPEM() string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.cert.Raw}))
}

// handshake runs a TLS handshake of client against a server presenting srv.
func handshake(t *testing.T, client *tls.Config, srv *testCert) error {
	t.Helper()
	pair, err := tls.X509KeyPair(srv.pem, srv.pem)
	require.NoError(t, err)
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	go func() {
		_ = tls.Server(b, &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12}).Handshake()
		_ = b.Close()
	}()
	return tls.Client(a, client).Handshake()
}

func TestClientTLS(t *testing.T) {
	ca := newCert(t, nil, "")
	srv := newCert(t, ca, "db.local")
	client := newCert(t, ca, "client")

	cfg, err := dbproxy.ClientTLS("disable", ca.certPEM(), "db.local", nil)
	require.NoError(t, err)
	assert.Nil(t, cfg, "disable sends plaintext")

	for _, mode := range []string{"", "verify-full"} {
		cfg, err = dbproxy.ClientTLS(mode, ca.certPEM(), "db.local", nil)
		require.NoError(t, err, mode)
		assert.False(t, cfg.InsecureSkipVerify, mode)
		assert.NotNil(t, cfg.RootCAs, mode)
		assert.Equal(t, "db.local", cfg.ServerName, mode)
		assert.GreaterOrEqual(t, cfg.MinVersion, uint16(tls.VersionTLS12), mode)
		require.NoError(t, handshake(t, cfg, srv), mode)
	}

	cfg, err = dbproxy.ClientTLS("verify-full", "", "db.local", nil)
	require.NoError(t, err)
	assert.Nil(t, cfg.RootCAs, "no ca_pem: the system roots")
	assert.Equal(t, "db.local", cfg.ServerName)

	cfg, err = dbproxy.ClientTLS("verify-full", client.certPEM(), "db.local", client.pem)
	require.NoError(t, err)
	require.Len(t, cfg.Certificates, 1, "the client certificate is presented")
}

func TestClientTLSVerifyFullRefuses(t *testing.T) {
	ca, other := newCert(t, nil, ""), newCert(t, nil, "")
	srv := newCert(t, ca, "db.local")

	cfg, err := dbproxy.ClientTLS("verify-full", other.certPEM(), "db.local", nil)
	require.NoError(t, err)
	require.ErrorContains(t, handshake(t, cfg, srv), "certificate", "a server of another CA is refused")

	cfg, err = dbproxy.ClientTLS("", ca.certPEM(), "other.local", nil)
	require.NoError(t, err)
	require.ErrorContains(t, handshake(t, cfg, srv), "certificate", "a server certificate for another host is refused")
}

func TestClientTLSVerifyCA(t *testing.T) {
	ca, other := newCert(t, nil, ""), newCert(t, nil, "")
	srv := newCert(t, ca, "db.local")

	cfg, err := dbproxy.ClientTLS("verify-ca", ca.certPEM(), "10.0.0.5", nil)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, cfg.MinVersion, uint16(tls.VersionTLS12))
	require.NoError(t, handshake(t, cfg, srv), "a certificate of ca_pem for another name is accepted")

	cfg, err = dbproxy.ClientTLS("verify-ca", other.certPEM(), "db.local", nil)
	require.NoError(t, err)
	require.ErrorContains(t, handshake(t, cfg, srv), "certificate", "a server of another CA is refused")

	inter := newCert(t, ca, "")
	leaf := newCert(t, inter, "db.local")
	chained := *leaf
	chained.pem = append([]byte(leaf.certPEM()+inter.certPEM()), leaf.pem[len(leaf.certPEM()):]...)
	cfg, err = dbproxy.ClientTLS("verify-ca", ca.certPEM(), "10.0.0.5", nil)
	require.NoError(t, err)
	require.NoError(t, handshake(t, cfg, &chained), "an intermediate sent by the server completes the chain")
	require.ErrorContains(t, handshake(t, cfg, leaf), "certificate", "without the intermediate the chain is broken")
}

func TestClientTLSErrors(t *testing.T) {
	ca := newCert(t, nil, "")
	client := newCert(t, ca, "client")

	_, err := dbproxy.ClientTLS("disable", "", "db.local", client.pem)
	require.EqualError(t, err, "client certificate requires TLS")

	_, err = dbproxy.ClientTLS("verify-full", "not a pem", "db.local", nil)
	require.Error(t, err, "an unusable ca_pem fails closed")

	_, err = dbproxy.ClientTLS("verify-full", ca.certPEM(), "db.local", []byte("not a pem"))
	require.Error(t, err, "an unusable client certificate fails closed")

	_, err = dbproxy.ClientTLS("verify-ca", "", "db.local", nil)
	require.Error(t, err, "verify-ca without ca_pem fails closed")

	_, err = dbproxy.ClientTLS("verify-ca", "not a pem", "db.local", nil)
	require.Error(t, err, "verify-ca with an unusable ca_pem fails closed")

	for _, mode := range []string{"prefer", "require"} {
		_, err = dbproxy.ClientTLS(mode, "", "db.local", nil)
		require.Error(t, err, "an unknown sslmode fails closed: %s", mode)
	}
}
