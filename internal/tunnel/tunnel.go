// Package tunnel serves gateway.v1.TunnelService: the mTLS control plane that
// gateway instances use to fetch policy snapshots and open sessions. It never
// imports domains; the composition root adapts them to the interfaces below.
package tunnel

import (
	"compress/gzip"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/google/uuid"
	gossh "golang.org/x/crypto/ssh"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/gen/gateway/v1/gatewayv1connect"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/platform/sshca"
	"github.com/werbot/werbot/internal/platform/tlsca"
)

// Tunnel timings and limits.
const (
	SnapshotDebounce   = 500 * time.Millisecond
	StreamMaxAge       = 30 * time.Minute
	StreamMaxAgeJitter = 0.2
	HeartbeatTimeout   = 45 * time.Second
	CompressMinBytes   = 1024
)

// Gateway is the authenticated caller: ID is the pool, InstanceID the process.
// CertSerial is the instance's current certificate serial.
type Gateway struct {
	ID, InstanceID, OrgID uuid.UUID
	CertSerial            string
}

// User is an organization member resolved for an SSH login.
type User struct {
	ID      uuid.UUID
	Login   string
	TeamIDs []uuid.UUID
	Agent   *AgentRef // set for an agent certificate; the user is its sponsor
}

// Secret is a revealed resource credential.
type Secret struct {
	Kind  string
	Value []byte
}

// ResourceHealth is one probe result of a resource.
type ResourceHealth struct {
	ResourceID uuid.UUID
	OK         bool
	Error      string
	CheckedAt  time.Time
	// AgentReadonlyStatus is a check of the agent_readonly credential
	// (empty: none) of check version AgentReadonlyVersion, with its findings.
	AgentReadonlyStatus, AgentReadonlyDetail string
	AgentReadonlyVersion                     int64
}

// Gateways authenticates instances by their verified client certificate.
type Gateways interface {
	Authenticate(ctx context.Context, gatewayID, instanceID uuid.UUID, certSerial string) (Gateway, error)
	Heartbeat(ctx context.Context, instanceID uuid.UUID, version string, connectors []string) error
	Renew(ctx context.Context, instanceID uuid.UUID, csrDER []byte) ([]byte, error)
}

// Users resolves SSH clients to organization members.
type Users interface {
	ByFingerprint(ctx context.Context, orgID uuid.UUID, fingerprint string) (User, error)
	ByLogin(ctx context.Context, orgID uuid.UUID, login string) (User, error)
	ByCert(ctx context.Context, orgID uuid.UUID, id sshca.Identity) (User, error) // a live CLI session of the certificate
	// ByWebTicket reads an unspent, unexpired stream ticket of the instance.
	ByWebTicket(ctx context.Context, orgID, instanceID, ticketID uuid.UUID) (User, WebTicket, error)
}

// Resources checks pool ownership and pins host keys.
type Resources interface {
	// OwnedBy reports whether the pool gatewayID serves the resource, and its type.
	OwnedBy(ctx context.Context, orgID, gatewayID, resourceID uuid.UUID) (resourceType string, owned bool, err error)
	PinHostKey(ctx context.Context, orgID, resourceID uuid.UUID, key gossh.PublicKey) error
}

// Credentials reveals resource secrets.
type Credentials interface {
	// Reveal gets the resource type from the server's own resource row; an
	// agent's session of a type with a query action gets the verified
	// agent_readonly credential, any other session the primary one.
	Reveal(ctx context.Context, orgID, resourceID uuid.UUID, resourceType string, agent bool) (Secret, error)
	// WriteCredential is the primary credential for one write of an open agent
	// session of gw, approved for the statement whose SHA-256 is opSHA256.
	WriteCredential(ctx context.Context, gw Gateway, sessionID, approvalID uuid.UUID, opSHA256 []byte) (Secret, error)
	// ProbeCredential is the agent_readonly credential of a resource of gw's
	// pool and its check version; configVersion is the resource's
	// configuration in gw's snapshot, which must still be the current one.
	ProbeCredential(ctx context.Context, gw Gateway, resourceID uuid.UUID, configVersion int64) (Secret, int64, error)
}

// Snapshots builds the policy snapshot of a pool in two steps, so a refresh
// reads the organization-wide part once for all the pools it rebuilds.
type Snapshots interface {
	// Org returns the part every pool snapshot of the organization shares.
	Org(ctx context.Context, orgID uuid.UUID) (*gatewayv1.Snapshot, error)
	// Pool returns org completed with the pool's own part; org stays unchanged.
	Pool(ctx context.Context, org *gatewayv1.Snapshot, orgID, gatewayID uuid.UUID) (*gatewayv1.Snapshot, error)
}

// Sessions records sessions per instance and the attempts gateways refused.
type Sessions interface {
	Start(ctx context.Context, s SessionStart) (uuid.UUID, error)
	End(ctx context.Context, orgID, instanceID, id uuid.UUID, reason string, c *Counters, r *SessionResult) error // nil c keeps the stored counters
	EndMissing(ctx context.Context, orgID, instanceID uuid.UUID, active []uuid.UUID, reason string) (int64, error)
	UpdateCounters(ctx context.Context, orgID, instanceID uuid.UUID, cs []Counters) error
	Deny(ctx context.Context, orgID uuid.UUID, ds []Denial) error
	// PendingTerminations lists the open sessions of the instance asked to end.
	PendingTerminations(ctx context.Context, orgID, instanceID uuid.UUID) ([]uuid.UUID, error)
}

// Health stores the probe results of an instance.
type Health interface {
	ReportHealth(ctx context.Context, orgID, instanceID uuid.UUID, hs []ResourceHealth) error
}

// Pools lists the online instances of a pool.
type Pools interface {
	OnlineInstances(ctx context.Context, gatewayID uuid.UUID) ([]uuid.UUID, error)
}

// RecordingIngest begins, stores the chunks of and closes recordings of a
// session served by instanceID; *session.Recordings implements it.
type RecordingIngest interface {
	// Begin opens the recording of a new session whose format is not asciicast.
	Begin(ctx context.Context, orgID, sessionID uuid.UUID) error
	IngestChunk(ctx context.Context, orgID, instanceID, sessionID uuid.UUID, seq int, hash, gz []byte) error
	FinishRecording(ctx context.Context, orgID, instanceID, sessionID uuid.UUID, chunks int, bytes int64, finalHash []byte) error
}

// Deps are the domain adapters the handler calls.
type Deps struct {
	Gateways    Gateways
	Users       Users
	Resources   Resources
	Credentials Credentials
	Snapshots   Snapshots
	Sessions    Sessions
	Recordings  RecordingIngest
	Health      Health
	Pools       Pools
	Approvals   Approvals // nil: no approval decisions are sent
	Agents      Agents    // nil: RecordDenial is unimplemented
	Relay       Bridger   // nil: Stream is unimplemented
	MFA         MFA       // nil: the MFA challenge RPCs are unimplemented, no MFA age is sent
	Country     func(netip.Addr) string
	Metrics     *metrics.Registry // nil: a private registry
	Logger      *slog.Logger      // nil: slog.Default()
}

// Handler implements gatewayv1connect.TunnelServiceHandler.
type Handler struct {
	gatewayv1connect.UnimplementedTunnelServiceHandler
	d       Deps
	hub     *Hub
	log     *slog.Logger
	streams *metrics.Gauge
	pushes  *metrics.Counter
	gz      gzipCache // Connect messages, gzipped once for all streams

	newBuilds newBuilds // snapshot builds shared by new streams
}

// New wires the handler and attaches hub to its snapshot builder and re-authentication.
func New(d Deps, hub *Hub) *Handler {
	if d.Metrics == nil {
		d.Metrics = metrics.NewRegistry()
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	h := &Handler{d: d, hub: hub, log: d.Logger,
		streams: d.Metrics.Gauge("werbot_tunnel_streams", "Open gateway tunnel streams on this replica."),
		pushes:  d.Metrics.Counter("werbot_snapshot_pushes_total", "Policy snapshots sent to gateways.")}
	hub.attach(h)
	return h
}

// Mount registers TunnelService behind the mTLS middleware. Connect has its
// own handler: its gzip compressor encodes each distinct message once for
// all streams (gzipCache); the other procedures keep the default one. Stream
// has its own read limit (mountStream).
func (h *Handler) Mount(mux *http.ServeMux) {
	opts := connect.WithHandlerOptions(connect.WithCompressMinBytes(CompressMinBytes),
		connect.WithReadMaxBytes(readMaxBytes), connect.WithInterceptors(validate.NewInterceptor()))
	path, handler := gatewayv1connect.NewTunnelServiceHandler(h, opts)
	mux.Handle(path, h.mtls(handler))
	stream := connect.NewBidiStreamHandler(gatewayv1connect.TunnelServiceConnectProcedure, h.Connect, opts,
		connect.WithSchema(gatewayv1.File_gateway_v1_tunnel_proto.Services().ByName("TunnelService").Methods().ByName("Connect")),
		connect.WithCompression("gzip", func() connect.Decompressor { return &gzip.Reader{} },
			func() connect.Compressor { return &gzipOnce{c: &h.gz} }))
	mux.Handle(gatewayv1connect.TunnelServiceConnectProcedure, h.mtls(stream))
	h.mountStream(mux)
}

// readMaxBytes caps one tunnel message; a snapshot only travels server → gateway.
// The worst valid heartbeat (console denials at their bounds) fits it.
const readMaxBytes = 4 << 20

type ctxKey int

const (
	gatewayKey ctxKey = iota
	serialKey
)

func withGateway(ctx context.Context, g Gateway) context.Context {
	return context.WithValue(ctx, gatewayKey, g)
}

func gatewayFrom(ctx context.Context) (Gateway, bool) {
	g, ok := ctx.Value(gatewayKey).(Gateway)
	return g, ok
}

// mtls authenticates the verified client certificate on every request: the
// CN names the pool and instance, the serial must match the enrolled one. A
// check is answered from the hub's cache until a change notification drops it.
func (h *Handler) mtls(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 || len(r.TLS.VerifiedChains[0]) == 0 {
			http.Error(w, "verified client certificate required", http.StatusUnauthorized)
			return
		}
		cert := r.TLS.VerifiedChains[0][0]
		gatewayID, instanceID, err := tlsca.ParseGatewayCN(cert.Subject.CommonName)
		if err != nil {
			http.Error(w, "unknown gateway", http.StatusUnauthorized)
			return
		}
		serial := cert.SerialNumber.Text(16)
		g, gen, ok := h.hub.auth.get(gatewayID, instanceID, serial)
		if !ok {
			if g, err = h.d.Gateways.Authenticate(r.Context(), gatewayID, instanceID, serial); err != nil {
				h.log.Warn("tunnel: gateway rejected", "gateway_id", gatewayID, "instance_id", instanceID, "error", err)
				if revoked(err) {
					http.Error(w, "unknown gateway", http.StatusUnauthorized)
				} else {
					http.Error(w, "try again later", http.StatusServiceUnavailable)
				}
				return
			}
			h.hub.auth.put(gen, serial, g)
		}
		ctx := context.WithValue(withGateway(r.Context(), g), serialKey, serial)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// revoked reports whether an Authenticate or Heartbeat error means the
// instance is gone (Unauthenticated) rather than a retryable failure (Unavailable).
func revoked(err error) bool {
	return errors.Is(err, apperr.ErrUnauthenticated) || errors.Is(err, apperr.ErrNotFound)
}

// authError is the stream error for a failed re-authentication.
func authError(err error) error {
	if revoked(err) {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("gateway is not authenticated"))
	}
	return unavailable("gateway authentication unavailable")
}

// caller returns the authenticated gateway; every handler takes org, pool and
// instance from here, never from the request.
func caller(ctx context.Context) (Gateway, error) {
	g, ok := gatewayFrom(ctx)
	if !ok {
		return Gateway{}, connect.NewError(connect.CodeUnauthenticated, apperr.ErrUnauthenticated)
	}
	return g, nil
}
