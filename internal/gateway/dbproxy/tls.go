package dbproxy

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
)

// ClientTLS is the TLS configuration of a connection to a database at host.
// sslmode "" and "verify-full" verify the server against caPEM (the system
// roots when empty) and host; "require" encrypts without verification;
// "disable" is plaintext (nil). clientCert is a PEM certificate and key in one
// value, or nil. Any other mode or unusable PEM is an error.
func ClientTLS(mode, caPEM, host string, clientCert []byte) (*tls.Config, error) {
	var cfg *tls.Config
	switch mode {
	case "disable":
		if len(clientCert) > 0 {
			return nil, errors.New("client certificate requires TLS")
		}
		return nil, nil
	case "require":
		cfg = &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12} //nolint:gosec // sslmode=require encrypts without verification by definition
	case "", "verify-full":
		cfg = &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
		if caPEM != "" {
			cfg.RootCAs = x509.NewCertPool()
			if !cfg.RootCAs.AppendCertsFromPEM([]byte(caPEM)) {
				return nil, errors.New("dbproxy: ca_pem holds no certificate")
			}
		}
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
