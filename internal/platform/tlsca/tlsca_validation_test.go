package tlsca

import (
	"context"
	"crypto/x509"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/platform/apperr"
)

func TestIssueClientRejectsBadCSRSignature(t *testing.T) {
	ca := newTestCA(t)
	csr := ed25519CSR(t, "x")
	csr[len(csr)-1] ^= 0xff
	if _, err := ca.IssueClient(csr, GatewayCN(uuid.New(), uuid.New()), time.Hour); !errors.Is(err, apperr.ErrInvalidArgument) {
		t.Fatalf("err = %v, want ErrInvalidArgument", err)
	}
}

func TestIssueClientValidatesArgsAndCapsTTL(t *testing.T) {
	ca := newTestCA(t)
	csr := ed25519CSR(t, "x")
	cn := GatewayCN(uuid.New(), uuid.New())
	if _, err := ca.IssueClient(csr, cn, 0); !errors.Is(err, apperr.ErrInvalidArgument) {
		t.Fatalf("ttl 0: err = %v", err)
	}
	if _, err := ca.IssueClient(csr, cn, -time.Hour); !errors.Is(err, apperr.ErrInvalidArgument) {
		t.Fatalf("negative ttl: err = %v", err)
	}
	if _, err := ca.IssueClient(csr, "", time.Hour); !errors.Is(err, apperr.ErrInvalidArgument) {
		t.Fatalf("empty cn: err = %v", err)
	}
	certPEM, err := ca.IssueClient(csr, cn, 100*365*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if got := parseCertPEM(t, certPEM).NotAfter; got.After(ca.cert.NotAfter) {
		t.Fatalf("NotAfter %v exceeds root %v", got, ca.cert.NotAfter)
	}
}

func TestLoadOrCreateRejectsKeyRootMismatch(t *testing.T) {
	ctx := context.Background()
	box := testBox(t, 1)
	a, b := &memStore{}, &memStore{}
	if _, err := LoadOrCreate(ctx, a, box); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreate(ctx, b, box); err != nil {
		t.Fatal(err)
	}
	mixed := *a.s
	mixed.CertPEM = b.s.CertPEM
	if _, err := LoadOrCreate(ctx, &memStore{s: &mixed}, box); err == nil {
		t.Fatal("want error when stored key does not match root")
	}
}

func TestServerTLSRequiresHosts(t *testing.T) {
	if _, err := newTestCA(t).ServerTLS(nil); err == nil {
		t.Fatal("want error for empty hosts")
	}
}

func TestFingerprintPEMRejectsNonCertificate(t *testing.T) {
	for name, in := range map[string][]byte{
		"garbage":    []byte("garbage"),
		"wrong type": []byte("-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n"),
	} {
		if _, err := FingerprintPEM(in); !errors.Is(err, apperr.ErrInvalidArgument) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestFingerprintPEMRejectsTrailingData(t *testing.T) {
	a, b := newTestCA(t).RootPEM(), newTestCA(t).RootPEM()
	if _, err := FingerprintPEM(append(slices.Clone(a), b...)); !errors.Is(err, apperr.ErrInvalidArgument) {
		t.Fatalf("two certs: err = %v, want ErrInvalidArgument", err)
	}
	if _, err := FingerprintPEM(append(slices.Clone(a), "junk"...)); !errors.Is(err, apperr.ErrInvalidArgument) {
		t.Fatalf("trailing junk: err = %v, want ErrInvalidArgument", err)
	}
	if _, err := FingerprintPEM(append(slices.Clone(a), " \n"...)); err != nil {
		t.Fatalf("trailing whitespace: %v", err)
	}
}

func TestServerTLSCapsLeafAtRoot(t *testing.T) {
	ca := newTestCA(t)
	ca.cert.NotAfter = time.Now().Add(time.Hour)
	cfg, err := ca.ServerTLS([]string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(cfg.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if leaf.NotAfter.After(ca.cert.NotAfter) {
		t.Fatalf("leaf NotAfter %v after root %v", leaf.NotAfter, ca.cert.NotAfter)
	}
}
