package gateway

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

const (
	keyFile     = "gateway.key"
	certFile    = "gateway.crt"
	caFile      = "ca.crt"
	hostKeyFile = "ssh_host_ed25519_key"
)

// certStore holds the live client certificate; Replace swaps it without a restart.
type certStore struct {
	dir string
	cur atomic.Pointer[tls.Certificate]
}

// loadCertStore reads gateway.crt and gateway.key from dir.
func loadCertStore(dir string) (*certStore, error) {
	c, err := tls.LoadX509KeyPair(filepath.Join(dir, certFile), filepath.Join(dir, keyFile))
	if err != nil {
		return nil, fmt.Errorf("load gateway certificate: %w", err)
	}
	s := &certStore{dir: dir}
	s.cur.Store(&c)
	return s, nil
}

// GetClientCertificate serves tls.Config.GetClientCertificate.
func (s *certStore) GetClientCertificate(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
	return s.cur.Load(), nil
}

// Leaf is the current client certificate.
func (s *certStore) Leaf() *x509.Certificate { return s.cur.Load().Leaf }

// Replace validates the pair, writes it atomically and swaps it in.
func (s *certStore) Replace(certPEM, keyPEM []byte) error {
	c, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("invalid gateway certificate: %w", err)
	}
	if err := writeAtomic(filepath.Join(s.dir, keyFile), keyPEM, 0o600); err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(s.dir, certFile), certPEM, 0o644); err != nil {
		return err
	}
	s.cur.Store(&c)
	return nil
}

// writeAtomic writes via tmp + rename so a crash never leaves a torn file.
func writeAtomic(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck // no-op after a successful rename
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// newCSR makes an ed25519 key (PKCS8 PEM) and a CSR for it; the server sets the subject.
func newCSR() (keyPEM, csrDER []byte, err error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}
	csrDER, err = x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: "werbot-gateway"}}, priv)
	if err != nil {
		return nil, nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), csrDER, nil
}

// renewAt is the renewal moment: 2/3 of the certificate lifetime.
func renewAt(leaf *x509.Certificate) time.Time {
	return leaf.NotBefore.Add(leaf.NotAfter.Sub(leaf.NotBefore) * 2 / 3)
}

// loadHostKey reads the pool host key; only ensureEnrolled writes it.
func loadHostKey(dataDir string) (gossh.Signer, error) {
	b, err := os.ReadFile(filepath.Join(dataDir, hostKeyFile)) //nolint:gosec // path is DataDir + fixed name
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", hostKeyFile, err)
	}
	s, err := gossh.ParsePrivateKey(b)
	if err != nil {
		return nil, errors.New("parse " + hostKeyFile + ": invalid key")
	}
	return s, nil
}
