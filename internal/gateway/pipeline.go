package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/platform/version"
)

// Errors of Pipeline.Open; the front turns them into user messages.
var (
	ErrNotReady        = errors.New("gateway is not ready")
	ErrBusy            = errors.New("gateway is busy")
	ErrDenied          = errors.New("access denied")
	ErrUnsupportedType = errors.New("resource type is not supported by this gateway")
	ErrUnsupportedKind = errors.New("session kind is not supported by the resource type")
)

// Identity is how the front identified the user: "ssh-key" and the SHA256 fingerprint.
type Identity struct{ Method, Value string }

// OpenRequest is what a front asks the pipeline to open.
type OpenRequest struct {
	Front    string
	Kind     Kind
	Identity Identity
	Resource string // resource name
	Target   string // KindStream
	ClientIP netip.Addr
	Window   Window // KindTerminal: initial size for the recorder
}

// control is the control plane of the pipeline; *tunnelClient implements it.
type control interface {
	AuthenticateUser(ctx context.Context, fingerprint string, ip netip.Addr) (User, error)
	StartSession(ctx context.Context, userID, resourceID uuid.UUID, ip netip.Addr) (uuid.UUID, Secret, error)
	EndSession(ctx context.Context, sessionID uuid.UUID, reason string) error
	PinHostKey(ctx context.Context, resourceID uuid.UUID, alg string, key []byte) error
	Policy() *Policy
	Connected() bool
}

// endCause cancels a session context; its text is the end reason.
type endCause string

func (c endCause) Error() string { return string(c) }

const (
	causeShutdown endCause = "gateway_shutdown"
	causeInternal endCause = "internal_error"
)

// Pipeline opens sessions against the control plane and runs them on connectors.
type Pipeline struct {
	ctl    control
	cfg    Config
	active *activeSessions
	log    *slog.Logger

	busy, notReady, panics *metrics.Counter

	draining atomic.Bool
	stop     context.Context // canceled at the end of drain: stops EndSession retries
	stopNow  context.CancelFunc
	retries  sync.WaitGroup

	backoffMin, backoffMax, flushTimeout time.Duration
}

// newPipeline builds a pipeline; nil cfg.Recording is NopRecording, nil cfg.Metrics a private registry.
func newPipeline(ctl control, cfg Config, logger *slog.Logger) *Pipeline {
	if cfg.Recording == nil {
		cfg.Recording = NopRecording
	}
	if cfg.Metrics == nil {
		cfg.Metrics = metrics.NewRegistry()
	}
	reg := cfg.Metrics
	const rejectedHelp = "Sessions refused before start, by reason."
	stop, stopNow := context.WithCancel(context.Background())
	return &Pipeline{
		ctl: ctl, cfg: cfg, log: logger, stop: stop, stopNow: stopNow,
		active:     newActiveSessions(reg.Gauge("werbot_gateway_sessions_active", "Sessions holding a MaxSessions slot.")),
		busy:       reg.Counter("werbot_gateway_sessions_rejected_total", rejectedHelp, "reason", "busy"),
		notReady:   reg.Counter("werbot_gateway_sessions_rejected_total", rejectedHelp, "reason", "not_ready"),
		panics:     reg.Counter("werbot_gateway_panics_total", "Connector panics recovered by the pipeline."),
		backoffMin: time.Second, backoffMax: 30 * time.Second, flushTimeout: 10 * time.Second,
	}
}

// Open checks the request in contract order and starts the session on the server.
// Any error frees the slot.
func (p *Pipeline) Open(ctx context.Context, r OpenRequest) (*Session, error) {
	if p.draining.Load() {
		return nil, p.reject(ErrNotReady)
	}
	if !p.active.reserve(p.cfg.MaxSessions) {
		return nil, p.reject(ErrBusy)
	}
	s, err := p.open(ctx, r)
	if err != nil {
		p.active.release()
		return nil, p.reject(err)
	}
	return s, nil
}

func (p *Pipeline) reject(err error) error {
	switch {
	case errors.Is(err, ErrBusy):
		p.busy.Inc()
	case errors.Is(err, ErrNotReady):
		p.notReady.Inc()
	}
	return err
}

func (p *Pipeline) open(ctx context.Context, r OpenRequest) (*Session, error) {
	pol := p.ctl.Policy()
	if pol == nil || !p.ctl.Connected() {
		return nil, ErrNotReady
	}
	u, err := p.ctl.AuthenticateUser(ctx, r.Identity.Value, r.ClientIP)
	if err != nil {
		return nil, controlErr(err)
	}
	res, ok := pol.Resource(r.Resource)
	if !ok {
		return nil, ErrDenied
	}
	if allow, _ := pol.Authorize(AccessRequest{User: u, Resource: res, IP: r.ClientIP}); !allow {
		return nil, ErrDenied
	}
	c, ok := LookupConnector(res.Type)
	if !ok {
		return nil, ErrUnsupportedType
	}
	if !slices.Contains(c.Kinds(), r.Kind) {
		return nil, ErrUnsupportedKind
	}
	id, secret, err := p.ctl.StartSession(ctx, u.ID, res.ID, r.ClientIP)
	if err != nil {
		return nil, controlErr(err)
	}
	p.active.add(id) // reported in heartbeats from now on, so the server keeps the session
	return &Session{
		ID: id, Kind: r.Kind, Front: r.Front, User: u, Resource: res, Secret: secret, ClientIP: r.ClientIP, Target: r.Target,
		PinKey:    func(alg string, key []byte) error { return p.ctl.PinHostKey(p.stop, res.ID, alg, key) },
		Recording: p.cfg.Recording(id, r.Window.Cols, r.Window.Rows),
	}, nil
}

// controlErr maps a control plane error: an outage is ErrNotReady, anything else ErrDenied.
func controlErr(err error) error {
	if unreachable(err) {
		return fmt.Errorf("%w: %w", ErrNotReady, err)
	}
	return fmt.Errorf("%w: %w", ErrDenied, err)
}

// Run serves s on c until it ends, then ends the session on the server, closes
// the recording and frees the slot. A connector panic ends only this session.
func (p *Pipeline) Run(ctx context.Context, s *Session, c Connector) error {
	sctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	p.active.bind(s.ID, cancel)
	err := p.serve(sctx, s, c, cancel)
	reason := endReason(sctx, err)
	if reason == "error" {
		p.log.Warn("session: connector failed", "session_id", s.ID, "type", c.Type(), "error", redact(err.Error(), s.Secret))
	}
	p.finish(ctx, s, reason)
	return err
}

func (p *Pipeline) serve(ctx context.Context, s *Session, c Connector, cancel context.CancelCauseFunc) (err error) {
	defer func() {
		if r := recover(); r != nil {
			cancel(causeInternal)
			p.panics.Inc()
			p.log.Error("session: connector panic", "session_id", s.ID, "type", c.Type(),
				"panic", redact(fmt.Sprint(r), s.Secret), "stack", redact(string(debug.Stack()), s.Secret))
			err = fmt.Errorf("connector %q panicked", c.Type())
		}
	}()
	return c.Serve(ctx, s)
}

// abort ends a session that Open returned but no connector ran.
func (p *Pipeline) abort(s *Session) { p.finish(context.Background(), s, "error") }

// finish sends EndSession before Recording.Close (contract §3.3), then frees the slot.
func (p *Pipeline) finish(ctx context.Context, s *Session, reason string) {
	p.endSession(s.ID, reason)
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := s.Recording.Close(cctx); err != nil {
		p.log.Warn("session: recording close failed", "session_id", s.ID, "error", err)
	}
	p.active.release()
}

// endReason is the cause the session context was canceled with; a normal end or
// a client cancel is "closed", a connector error "error".
func endReason(ctx context.Context, err error) string {
	var c endCause
	if errors.As(context.Cause(ctx), &c) {
		return string(c)
	}
	if err == nil || ctx.Err() != nil {
		return "closed"
	}
	return "error"
}

// redact removes the credential value from text bound for the log.
func redact(text string, s Secret) string {
	if len(s.Value) == 0 {
		return text
	}
	return strings.ReplaceAll(text, string(s.Value), "[redacted]")
}

// heartbeat is the instance state sent on the tunnel.
func (p *Pipeline) heartbeat() *gatewayv1.Heartbeat {
	return &gatewayv1.Heartbeat{Version: version.Version, Connectors: connectors(), ActiveSessionIds: p.active.ids()}
}

// ready is the /readyz state: a reason is "not_ready", "busy" or "draining".
func (p *Pipeline) ready() (bool, string) {
	switch {
	case p.draining.Load():
		return false, "draining"
	case p.ctl.Policy() == nil || !p.ctl.Connected():
		return false, "not_ready"
	case p.active.used() >= p.cfg.MaxSessions:
		return false, "busy"
	}
	return true, ""
}
