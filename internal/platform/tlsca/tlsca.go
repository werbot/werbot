// Package tlsca is the internal ed25519 CA that secures gateway-to-core mTLS.
package tlsca

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/secretbox"
)

const (
	rootTTL   = 10 * 365 * 24 * time.Hour
	serverTTL = 365 * 24 * time.Hour
	skew      = time.Minute
)

// CA signs gateway client certificates and server leaf certificates.
type CA struct {
	key     ed25519.PrivateKey
	cert    *x509.Certificate
	rootPEM []byte
}

// LoadOrCreate returns the stored CA, creating it on first use.
// Call only under db.BootstrapLock; ON CONFLICT in Insert is a safety net.
func LoadOrCreate(ctx context.Context, store CAStore, box *secretbox.Box) (*CA, error) {
	st, ok, err := store.Load(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		if st, err = newStored(box); err != nil {
			return nil, err
		}
		if err := store.Insert(ctx, st); err != nil {
			return nil, err
		}
		if st, ok, err = store.Load(ctx); err != nil {
			return nil, err
		} else if !ok {
			return nil, errors.New("tlsca: CA missing after insert")
		}
	}
	return open(st, box)
}

func newStored(box *secretbox.Box) (StoredCA, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return StoredCA{}, fmt.Errorf("tlsca: generate key: %w", err)
	}
	serial, err := serialNumber()
	if err != nil {
		return StoredCA{}, err
	}
	now := time.Now()
	tpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "Werbot Internal CA"},
		NotBefore:             now.Add(-skew),
		NotAfter:              now.Add(rootTTL),
		IsCA:                  true,
		BasicConstraintsValid: true,
		MaxPathLenZero:        true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, pub, priv)
	if err != nil {
		return StoredCA{}, fmt.Errorf("tlsca: self-sign root: %w", err)
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return StoredCA{}, fmt.Errorf("tlsca: marshal key: %w", err)
	}
	ct, keyID, err := box.Seal(pkcs8)
	if err != nil {
		return StoredCA{}, fmt.Errorf("tlsca: seal key: %w", err)
	}
	return StoredCA{Ciphertext: ct, KeyID: keyID, CertPEM: pemCert(der)}, nil
}

func open(st StoredCA, box *secretbox.Box) (*CA, error) {
	pkcs8, err := box.Open(st.Ciphertext, st.KeyID)
	if err != nil {
		return nil, fmt.Errorf("tlsca: open key: %w", err)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(pkcs8)
	if err != nil {
		return nil, errors.New("tlsca: stored key is not valid PKCS#8")
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("tlsca: stored key is not ed25519")
	}
	blk, _ := pem.Decode(st.CertPEM)
	if blk == nil {
		return nil, errors.New("tlsca: stored root is not PEM")
	}
	cert, err := x509.ParseCertificate(blk.Bytes)
	if err != nil {
		return nil, fmt.Errorf("tlsca: parse root: %w", err)
	}
	if pub, ok := cert.PublicKey.(ed25519.PublicKey); !ok || !pub.Equal(key.Public()) {
		return nil, errors.New("tlsca: stored key does not match root certificate")
	}
	return &CA{key: key, cert: cert, rootPEM: st.CertPEM}, nil
}

// IssueClient signs a client-auth certificate for the CSR public key.
// The CSR subject is ignored: the certificate carries commonName.
func (c *CA) IssueClient(csrDER []byte, commonName string, ttl time.Duration) ([]byte, error) {
	if ttl <= 0 || commonName == "" {
		return nil, fmt.Errorf("%w: ttl and common name are required", apperr.ErrInvalidArgument)
	}
	now := time.Now()
	return c.issue(csrDER, commonName, now.Add(-skew), minTime(now.Add(ttl), c.cert.NotAfter))
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

func (c *CA) issue(csrDER []byte, cn string, notBefore, notAfter time.Time) ([]byte, error) {
	csr, err := x509.ParseCertificateRequest(csrDER)
	if err != nil {
		return nil, fmt.Errorf("%w: parse CSR", apperr.ErrInvalidArgument)
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, fmt.Errorf("%w: CSR signature", apperr.ErrInvalidArgument)
	}
	pub, ok := csr.PublicKey.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("%w: CSR key must be ed25519", apperr.ErrInvalidArgument)
	}
	return c.sign(pub, cn, notBefore, notAfter, x509.ExtKeyUsageClientAuth, nil, nil)
}

func (c *CA) sign(pub ed25519.PublicKey, cn string, notBefore, notAfter time.Time, usage x509.ExtKeyUsage, ips []net.IP, dns []string) ([]byte, error) {
	serial, err := serialNumber()
	if err != nil {
		return nil, err
	}
	tpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{usage},
		IPAddresses:  ips,
		DNSNames:     dns,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, c.cert, pub, c.key)
	if err != nil {
		return nil, fmt.Errorf("tlsca: sign certificate: %w", err)
	}
	return pemCert(der), nil
}

// ServerTLS returns a TLS 1.3 config with a fresh leaf for hosts that demands
// a client certificate issued by this CA.
func (c *CA) ServerTLS(hosts []string) (*tls.Config, error) {
	if len(hosts) == 0 {
		return nil, fmt.Errorf("%w: server hosts are required", apperr.ErrInvalidArgument)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("tlsca: generate server key: %w", err)
	}
	var ips []net.IP
	var dns []string
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			ips = append(ips, ip)
		} else {
			dns = append(dns, h)
		}
	}
	now := time.Now()
	certPEM, err := c.sign(pub, "werbot-core", now.Add(-skew), minTime(now.Add(serverTTL), c.cert.NotAfter), x509.ExtKeyUsageServerAuth, ips, dns)
	if err != nil {
		return nil, err
	}
	blk, _ := pem.Decode(certPEM)
	roots := x509.NewCertPool()
	roots.AddCert(c.cert)
	return &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{blk.Bytes}, PrivateKey: priv}},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    roots,
		MinVersion:   tls.VersionTLS13,
		NextProtos:   []string{"h2"},
	}, nil
}

// RootPEM returns the root certificate in PEM form.
func (c *CA) RootPEM() []byte { return c.rootPEM }

// Fingerprint is the lowercase hex SHA-256 of the root certificate DER.
func (c *CA) Fingerprint() string { return fingerprint(c.cert.Raw) }

// FingerprintPEM computes Fingerprint for a root certificate in PEM form.
// The input must be exactly one CERTIFICATE block: anything else after it is rejected,
// so a pinned fingerprint covers every byte that a caller may later trust.
func FingerprintPEM(rootPEM []byte) (string, error) {
	blk, rest := pem.Decode(rootPEM)
	if blk == nil || blk.Type != "CERTIFICATE" {
		return "", fmt.Errorf("%w: root is not a PEM certificate", apperr.ErrInvalidArgument)
	}
	if len(bytes.TrimSpace(rest)) != 0 {
		return "", fmt.Errorf("%w: root must be exactly one certificate", apperr.ErrInvalidArgument)
	}
	return fingerprint(blk.Bytes), nil
}

func fingerprint(der []byte) string {
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}

// GatewayCN is the client certificate common name of one gateway instance.
func GatewayCN(gatewayID, instanceID uuid.UUID) string {
	return "gateway:" + gatewayID.String() + ":" + instanceID.String()
}

// ParseGatewayCN reverses GatewayCN.
func ParseGatewayCN(cn string) (gatewayID, instanceID uuid.UUID, err error) {
	parts := strings.Split(cn, ":")
	if len(parts) != 3 || parts[0] != "gateway" {
		return uuid.Nil, uuid.Nil, fmt.Errorf("%w: bad gateway common name", apperr.ErrInvalidArgument)
	}
	if gatewayID, err = uuid.Parse(parts[1]); err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("%w: bad gateway id", apperr.ErrInvalidArgument)
	}
	if instanceID, err = uuid.Parse(parts[2]); err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("%w: bad instance id", apperr.ErrInvalidArgument)
	}
	if GatewayCN(gatewayID, instanceID) != cn {
		return uuid.Nil, uuid.Nil, fmt.Errorf("%w: non-canonical gateway common name", apperr.ErrInvalidArgument)
	}
	return gatewayID, instanceID, nil
}

func pemCert(der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func serialNumber() (*big.Int, error) {
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("tlsca: serial number: %w", err)
	}
	return n, nil
}
