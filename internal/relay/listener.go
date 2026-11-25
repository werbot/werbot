package relay

import (
	"crypto/tls"
	"encoding/pem"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/werbot/werbot/internal/platform/tlsca"
)

// clientCertTTL bounds a replica client certificate; one is issued per TLS
// handshake, so an HTTP/2 connection may outlive it (checked only at handshake).
const clientCertTTL = time.Hour

// server is the internal listener: TLS 1.3, HTTP/2 only, client certificates
// of the internal CA, a leaf for the host of InternalURL.
func (r *Relay) server() (*http.Server, error) {
	u, err := url.Parse(r.cfg.InternalURL)
	if err != nil || u.Hostname() == "" {
		return nil, errors.New("relay: WERBOT_INTERNAL_URL: want https://host")
	}
	tlsCfg, err := r.ca.ServerTLS([]string{u.Hostname()})
	if err != nil {
		return nil, err
	}
	var protos http.Protocols
	protos.SetHTTP2(true)
	return &http.Server{
		Handler:           r.internalHandler(r.mux),
		TLSConfig:         tlsCfg,
		Protocols:         &protos,
		ReadHeaderTimeout: 10 * time.Second,
		// MaxReadFrameSize: a bridging client sizes each stream's write buffer
		// by it (512 KB at the 1 MB default, 16 KB here)
		HTTP2: &http.HTTP2Config{SendPingTimeout: 5 * time.Second, PingTimeout: 5 * time.Second,
			MaxReceiveBufferPerStream: 64 << 10, MaxConcurrentStreams: 1000, MaxReadFrameSize: 16 << 10},
		ErrorLog: slog.NewLogLogger(r.log.Handler(), slog.LevelDebug), // failed handshakes
	}, nil
}

// internalHandler admits only a verified client certificate whose common name
// is the canonical ServerCN of a replica with a server_instance row; a gateway
// certificate of the same CA and an unknown replica get 401.
func (r *Relay) internalHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.TLS == nil || len(req.TLS.VerifiedChains) == 0 || len(req.TLS.VerifiedChains[0]) == 0 {
			http.Error(w, "verified client certificate required", http.StatusUnauthorized)
			return
		}
		id, err := tlsca.ParseServerCN(req.TLS.VerifiedChains[0][0].Subject.CommonName)
		if err != nil {
			http.Error(w, "unknown replica", http.StatusUnauthorized)
			return
		}
		ok, err := r.known(req.Context(), id)
		if err != nil {
			r.log.Warn("relay: replica check failed", "instance_id", id, "error", err)
			http.Error(w, "try again later", http.StatusServiceUnavailable)
			return
		}
		if !ok {
			r.log.Warn("relay: replica rejected", "instance_id", id)
			http.Error(w, "unknown replica", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, req)
	})
}

// clientCert issues this replica's client certificate (CN ServerCN, 1h) for
// one TLS handshake; the key never leaves the process.
func (r *Relay) clientCert(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
	certPEM, err := r.ca.IssueClient(r.csr, tlsca.ServerCN(r.cfg.InstanceID), clientCertTTL)
	if err != nil {
		return nil, err
	}
	blk, _ := pem.Decode(certPEM)
	if blk == nil {
		return nil, errors.New("relay: issued certificate is not PEM")
	}
	return &tls.Certificate{Certificate: [][]byte{blk.Bytes}, PrivateKey: r.key}, nil
}
