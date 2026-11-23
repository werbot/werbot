// Package relay connects server replicas: it keeps this replica in the
// server_instance registry and serves the replica-to-replica mTLS listener.
// It imports no domain package; domains reach it through interfaces wired in
// internal/app.
package relay

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/internal/platform/tlsca"
)

// Registry timings.
const (
	HeartbeatInterval = 15 * time.Second // last_seen_at refresh
	OnlineWindow      = 45 * time.Second // a replica seen within it is online
	InstanceGCAfter   = 10 * time.Minute // rows older than this are deleted by GC
	GCInterval        = 5 * time.Minute  // server_instance.gc period
)

// Config identifies this replica.
type Config struct {
	InstanceID   uuid.UUID
	InternalAddr string // WERBOT_INTERNAL_ADDR
	InternalURL  string // WERBOT_INTERNAL_URL
	Version      string
}

// Relay is one server replica in the cluster.
type Relay struct {
	pool *pgxpool.Pool
	ca   *tlsca.CA
	cfg  Config
	log  *slog.Logger
	mux  *http.ServeMux // internal listener routes, behind internalHandler

	key ed25519.PrivateKey // client key of this replica, in memory only
	csr []byte             // its CSR, signed again on every TLS handshake

	heartbeat  time.Duration
	registered atomic.Bool // set after the first server_instance upsert
}

// New builds the relay of replica cfg.InstanceID; Run starts it.
func New(pool *pgxpool.Pool, ca *tlsca.CA, cfg Config, logger *slog.Logger) *Relay {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic("relay: generate client key: " + err.Error()) // crypto/rand never fails
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, key)
	if err != nil {
		panic("relay: client CSR: " + err.Error()) // fixed template and key type
	}
	return &Relay{pool: pool, ca: ca, cfg: cfg, log: logger, mux: http.NewServeMux(),
		key: key, csr: csr, heartbeat: HeartbeatInterval}
}

// Run binds the internal listener, registers the replica, keeps it alive and
// serves until ctx ends; then it stops the listener and removes the row.
func (r *Relay) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", r.cfg.InternalAddr)
	if err != nil {
		return err
	}
	host, _ := os.Hostname() // shown only; "" when unknown
	if err := r.register(ctx, host); err != nil {
		_ = ln.Close()
		return err
	}
	defer r.deregister(ctx)
	srv, err := r.server()
	if err != nil {
		_ = ln.Close()
		return err
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ServeTLS(ln, "", "") }()
	return r.keepAlive(ctx, host, srv, errc)
}

// keepAlive refreshes the registry row every heartbeat until ctx ends or the
// server fails, then shuts the server down within 5s.
func (r *Relay) keepAlive(ctx context.Context, host string, srv *http.Server, errc <-chan error) error {
	t := time.NewTicker(r.heartbeat)
	defer t.Stop()
	for {
		select {
		case err := <-errc:
			return err
		case <-t.C:
			if err := r.register(ctx, host); err != nil && ctx.Err() == nil {
				r.log.Warn("relay: heartbeat failed", "error", err)
			}
		case <-ctx.Done():
			sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(sctx)
			<-errc
			return nil
		}
	}
}
