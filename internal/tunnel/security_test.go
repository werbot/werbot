package tunnel

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/tlsca"
)

func TestRejectsUnverifiedCertificate(t *testing.T) {
	// The server accepts any client certificate, so only the middleware stands guard.
	b := newBenchTLS(t, nil, func(c *tls.Config) { c.ClientAuth, c.ClientCAs = tls.RequireAnyClientCert, nil })
	victim := b.instance(t, b.org, uuid.New())

	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	serial, _ := new(big.Int).SetString(victim.serial, 16)
	tmpl := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: tlsca.GatewayCN(victim.ID, victim.InstanceID)},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, priv.Public(), priv)
	if err != nil {
		t.Fatal(err)
	}
	forged := &tls.Certificate{Certificate: [][]byte{der}, PrivateKey: priv}
	attacker := b.instanceWith(t, b.org, uuid.New(), func(rt http.RoundTripper) http.RoundTripper {
		tc := rt.(*http.Transport).TLSClientConfig
		tc.Certificates = nil
		tc.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) { return forged, nil }
		return rt
	})
	b.f.with(func(f *fake) { f.auths = nil })

	_, err = attacker.client.RenewCertificate(ctx, connect.NewRequest(&gatewayv1.RenewCertificateRequest{CsrDer: []byte("CSR")}))
	wantCode(t, err, connect.CodeUnauthenticated, "")
	b.f.with(func(f *fake) {
		if len(f.auths) != 0 || len(f.renewed) != 0 {
			t.Fatal("a self-signed certificate reached Authenticate")
		}
	})
}

func TestAuthenticateErrorCodes(t *testing.T) {
	b := newBench(t, func(h *Hub) { h.debounce = 20 * time.Millisecond })
	g := b.instance(t, b.org, uuid.New())
	renew := func() error {
		_, err := g.client.RenewCertificate(ctx, connect.NewRequest(&gatewayv1.RenewCertificateRequest{CsrDer: []byte("CSR")}))
		return err
	}
	for err, code := range map[error]connect.Code{
		apperr.ErrUnauthenticated: connect.CodeUnauthenticated,
		apperr.ErrNotFound:        connect.CodeUnauthenticated,
		apperr.ErrUnavailable:     connect.CodeUnavailable,
	} {
		b.f.with(func(f *fake) { f.authErr[g.InstanceID] = err })
		wantCode(t, renew(), code, "")
	}

	// Hub refresh: a transient failure closes the stream as retryable.
	b.f.with(func(f *fake) { delete(f.authErr, g.InstanceID) })
	c := g.connect(t, nil)
	c.next(t, 2*time.Second)
	b.f.with(func(f *fake) { f.authErr[g.InstanceID] = apperr.ErrUnavailable })
	b.hub.OrgChanged(b.org.String())
	wantCode(t, c.closed(t, 2*time.Second), connect.CodeUnavailable, "")
}

func TestRequestLimits(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	renew := func(csr []byte) error {
		_, err := g.client.RenewCertificate(ctx, connect.NewRequest(&gatewayv1.RenewCertificateRequest{CsrDer: csr}))
		return err
	}
	wantCode(t, renew(nil), connect.CodeInvalidArgument, "")
	wantCode(t, renew(make([]byte, 4097)), connect.CodeInvalidArgument, "")
	wantCode(t, renew(make([]byte, 2<<20)), connect.CodeResourceExhausted, "")

	_, err := g.client.EndSession(ctx, connect.NewRequest(&gatewayv1.EndSessionRequest{SessionId: uuid.NewString(), Reason: "gateway_lost"}))
	wantCode(t, err, connect.CodeInvalidArgument, "")
	_, err = g.client.AuthenticateUser(ctx, connect.NewRequest(&gatewayv1.AuthenticateUserRequest{ClientIp: "10.0.0.1"}))
	wantCode(t, err, connect.CodeInvalidArgument, "")

	for name, hb := range map[string]*gatewayv1.Heartbeat{
		"version":    {Version: strings.Repeat("v", 65)},
		"session id": {Version: "1.0", ActiveSessionIds: []string{"not-a-uuid"}},
		"connectors": {Version: "1.0", Connectors: make([]string, 65)},
	} {
		c := g.connect(t, hb)
		if code := connect.CodeOf(c.closed(t, 2*time.Second)); code != connect.CodeInvalidArgument {
			t.Fatalf("%s: stream ended with %v, want invalid_argument", name, code)
		}
	}
	b.f.with(func(f *fake) {
		if len(f.ends) != 0 || len(f.hbs) != 0 || len(f.renewed) != 0 {
			t.Fatal("an invalid request reached a dependency")
		}
	})
}
