// Package relay connects server replicas: it keeps this replica in the
// server_instance registry, serves the replica-to-replica mTLS listener and
// bridges gateway streams to the replica holding the browser's WebSocket.
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
	"sync"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/gen/werbot/relay/v1/relayv1connect"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/platform/tlsca"
)

// Registry timings.
const (
	HeartbeatInterval = 15 * time.Second // last_seen_at refresh
	OnlineWindow      = 45 * time.Second // a replica seen within it is online
	InstanceGCAfter   = 10 * time.Minute // rows older than this are deleted by GC
	GCInterval        = 5 * time.Minute  // server_instance.gc period
)

// Stream limits.
const (
	AwaitTimeout = 10 * time.Second // a gateway opens a browser stream within it
	MaxStreams   = 1000             // browser WebSockets plus remote bridges per replica
)

// Config identifies this replica.
type Config struct {
	InstanceID   uuid.UUID
	InternalAddr string // WERBOT_INTERNAL_ADDR
	InternalURL  string // WERBOT_INTERNAL_URL
	Version      string
	Metrics      *metrics.Registry // nil: a private registry
}

// Ticket is a spent stream ticket: the gateway instance it is addressed to
// and the replica holding the browser.
type Ticket struct {
	ID, OrgID, UserID, InstanceID, TargetServerID uuid.UUID
	Mode                                          string
	SessionID, ResourceID                         *uuid.UUID
	Cols, Rows                                    int
}

// Tickets issues and spends stream tickets; session implements it through
// an adapter in internal/app. Consume is the only spend: an unknown, expired
// or spent ticket is apperr.ErrNotFound.
type Tickets interface {
	CreateObserve(ctx context.Context, p principal.Principal, sessionID uuid.UUID, target uuid.UUID) (ticketID, instanceID uuid.UUID, err error)
	CreateTerminal(ctx context.Context, p principal.Principal, resource string, cols, rows int, target uuid.UUID) (ticketID, instanceID uuid.UUID, err error)
	Consume(ctx context.Context, ticketID uuid.UUID) (Ticket, error)
}

// Relay is one server replica in the cluster.
type Relay struct {
	pool    *pgxpool.Pool
	ca      *tlsca.CA
	tickets Tickets
	cfg     Config
	log     *slog.Logger
	reg     *metrics.Registry
	slots   *metrics.Gauge  // werbot_relay_streams: taken slots
	mux     *http.ServeMux  // internal listener routes, behind internalHandler
	tr      *http.Transport // client of the other replicas' listeners

	key ed25519.PrivateKey // client key of this replica, in memory only
	csr []byte             // its CSR, signed again on every TLS handshake

	heartbeat    time.Duration
	awaitTimeout time.Duration
	maxStreams   int
	registered   atomic.Bool  // set after the first server_instance upsert
	active       atomic.Int64 // taken stream slots

	mu    sync.Mutex
	waits map[uuid.UUID]chan *stream // browsers waiting for their ticket's stream
	live  map[closer]struct{}        // streams closed on shutdown
}

// closer is a live stream the relay ends on shutdown.
type closer interface{ Close(reason string) error }

// New builds the relay of replica cfg.InstanceID; Run starts it.
func New(pool *pgxpool.Pool, ca *tlsca.CA, tickets Tickets, cfg Config, logger *slog.Logger) *Relay {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic("relay: generate client key: " + err.Error()) // crypto/rand never fails
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, key)
	if err != nil {
		panic("relay: client CSR: " + err.Error()) // fixed template and key type
	}
	if cfg.Metrics == nil {
		cfg.Metrics = metrics.NewRegistry()
	}
	r := &Relay{pool: pool, ca: ca, tickets: tickets, cfg: cfg, log: logger, reg: cfg.Metrics, mux: http.NewServeMux(),
		key: key, csr: csr, heartbeat: HeartbeatInterval, awaitTimeout: AwaitTimeout, maxStreams: MaxStreams,
		waits: map[uuid.UUID]chan *stream{}, live: map[closer]struct{}{}}
	r.slots = r.reg.Gauge("werbot_relay_streams", "Browser streams and remote bridges held by this replica.")
	r.tr = r.transport()
	r.mux.Handle(relayv1connect.NewRelayServiceHandler(pipeService{r},
		connect.WithReadMaxBytes(pipeReadMaxBytes), connect.WithInterceptors(validate.NewInterceptor())))
	return r
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
			r.registered.Store(false)
			r.closeAll()
			sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(sctx)
			<-errc
			return nil
		}
	}
}
