package dbproxy

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
)

// ClientTLS is the TLS configuration of a connection to a database at host.
// sslmode "" and "verify-full" verify the server against caPEM (the system
// roots when empty) and host; "verify-ca" verifies the server chain against
// caPEM (required) but not the host name; "disable" is plaintext (nil).
// clientCert is a PEM certificate and key in one value, or nil. Any other
// mode or unusable PEM is an error.
func ClientTLS(mode, caPEM, host string, clientCert []byte) (*tls.Config, error) {
	if mode == "disable" {
		if len(clientCert) > 0 {
			return nil, errors.New("client certificate requires TLS")
		}
		return nil, nil
	}
	roots, err := caPool(caPEM)
	if err != nil {
		return nil, err
	}
	var cfg *tls.Config
	switch mode {
	case "", "verify-full":
		cfg = &tls.Config{ServerName: host, RootCAs: roots, MinVersion: tls.VersionTLS12}
	case "verify-ca":
		if roots == nil {
			return nil, errors.New("dbproxy: verify-ca needs ca_pem")
		}
		cfg = verifyCAConfig(roots, host)
	default:
		return nil, fmt.Errorf("dbproxy: unknown sslmode %q", mode)
	}
	if len(clientCert) > 0 {
		pair, err := tls.X509KeyPair(clientCert, clientCert)
		if err != nil {
			return nil, fmt.Errorf("dbproxy: client certificate: %w", err)
		}
		cfg.Certificates = []tls.Certificate{pair}
	}
	return cfg, nil
}

// caPool is the pool of caPEM, nil when it is empty.
func caPool(caPEM string) (*x509.CertPool, error) {
	if caPEM == "" {
		return nil, nil
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(caPEM)) {
		return nil, errors.New("dbproxy: ca_pem holds no certificate")
	}
	return pool, nil
}

// verifyCAConfig verifies the server chain against roots without the host
// name: crypto/tls checks either both or none, so the built-in check is off
// and VerifyConnection (run on resumed sessions too) does the chain. host
// is only the SNI.
func verifyCAConfig(roots *x509.CertPool, host string) *tls.Config {
	return &tls.Config{
		ServerName: host, MinVersion: tls.VersionTLS12,
		InsecureSkipVerify: true, //nolint:gosec // VerifyConnection verifies the chain; verify-ca skips only the host name
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return errors.New("dbproxy: the server sent no certificate")
			}
			inter := x509.NewCertPool()
			for _, c := range cs.PeerCertificates[1:] {
				inter.AddCert(c)
			}
			_, err := cs.PeerCertificates[0].Verify(x509.VerifyOptions{
				Roots: roots, Intermediates: inter, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			})
			return err
		},
	}
}
