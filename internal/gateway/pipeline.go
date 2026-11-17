package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"runtime/debug"
	"slices"
	"time"

	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/gateway/recorder"
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
	ErrSpoolMax        = errors.New("gateway: recording spool is full") // the user sees the ErrBusy text
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
	spool  *recorder.Spool // session ends and recordings on their way to the server; set by serve
	log    *slog.Logger

	busy, notReady, spoolFull, panics *metrics.Counter

	stop    context.Context // canceled at the end of drain: the front closes its connections
	stopNow context.CancelFunc

	flushTimeout time.Duration
}

// newPipeline builds a pipeline; nil cfg.Recording is NopRecording, nil cfg.Metrics a
// private registry, zero cfg.SpoolMax DefaultSpoolMax. The caller sets p.spool.
func newPipeline(ctl control, cfg Config, logger *slog.Logger) *Pipeline {
	if cfg.Recording == nil {
		cfg.Recording = NopRecording
	}
	if cfg.Metrics == nil {
		cfg.Metrics = metrics.NewRegistry()
	}
	if cfg.SpoolMax == 0 {
		cfg.SpoolMax = DefaultSpoolMax
	}
	reg := cfg.Metrics
	const rejectedHelp = "Sessions refused before start, by reason."
	stop, stopNow := context.WithCancel(context.Background())
	return &Pipeline{
		ctl: ctl, cfg: cfg, log: logger, stop: stop, stopNow: stopNow,
		active:       newActiveSessions(reg.Gauge("werbot_gateway_sessions_active", "Sessions holding a MaxSessions slot."), logger),
		busy:         reg.Counter("werbot_gateway_sessions_rejected_total", rejectedHelp, "reason", "busy"),
		notReady:     reg.Counter("werbot_gateway_sessions_rejected_total", rejectedHelp, "reason", "not_ready"),
		spoolFull:    reg.Counter("werbot_gateway_sessions_rejected_total", rejectedHelp, "reason", "spool_full"),
		panics:       reg.Counter("werbot_gateway_panics_total", "Connector panics recovered by the pipeline."),
		flushTimeout: 10 * time.Second,
	}
}

// Open checks the request in contract order and starts the session on the server.
// Any error frees the slot. A spool at SpoolMax refuses every new session.
func (p *Pipeline) Open(ctx context.Context, r OpenRequest) (*Session, error) {
	if p.spool.Total() >= p.cfg.SpoolMax {
		p.spoolFull.Inc()
		return nil, ErrSpoolMax
	}
	if err := p.active.reserve(p.cfg.MaxSessions); err != nil {
		return nil, p.reject(err)
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

// Run serves s on c until it ends, then spools the session end, closes the
// recording and frees the slot. A connector panic ends only this session.
func (p *Pipeline) Run(ctx context.Context, s *Session, c Connector) error {
	sctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	p.active.bind(s.ID, cancel)
	panicked, err := p.serve(sctx, s, c, cancel)
	reason := endReason(sctx, err)
	if panicked { // also after a client cancel or drain
		reason = string(causeInternal)
	}
	if reason == "error" {
		p.log.Warn("session: connector failed", "session_id", s.ID, "type", c.Type(), "error", s.Secret.Redact(err.Error()))
	}
	p.finish(ctx, s, reason)
	return err
}

func (p *Pipeline) serve(ctx context.Context, s *Session, c Connector, cancel context.CancelCauseFunc) (panicked bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			cancel(causeInternal)
			p.panics.Inc()
			p.log.Error("session: connector panic", "session_id", s.ID, "type", c.Type(),
				"panic", s.Secret.Redact(fmt.Sprint(r)), "stack", s.Secret.Redact(string(debug.Stack())))
			panicked, err = true, fmt.Errorf("connector %q panicked", c.Type())
		}
	}()
	return false, c.Serve(ctx, s)
}

// abort ends a session that Open returned but no connector ran.
func (p *Pipeline) abort(s *Session) { p.finish(context.Background(), s, "error") }

// finish spools EndSession before Recording.Close (contract §3.3): end.json comes
// before the recording is closed, so FinishRecording never overtakes the end, and the
// id stays in the heartbeat (active or a pending end) throughout. Then it frees the slot.
func (p *Pipeline) finish(ctx context.Context, s *Session, reason string) {
	if err := p.spool.End(s.ID, reason); err != nil {
		p.log.Error("session: end not spooled; the server closes it as lost", "session_id", s.ID, "reason", reason, "error", err)
	}
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := s.Recording.Close(cctx); err != nil {
		p.log.Warn("session: recording close failed", "session_id", s.ID, "error", err)
	}
	p.active.done(s.ID)
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

// heartbeat is the instance state sent on the tunnel.
func (p *Pipeline) heartbeat() *gatewayv1.Heartbeat {
	return &gatewayv1.Heartbeat{Version: version.Version, Connectors: connectors(), ActiveSessionIds: heartbeatSessionIDs(p.active, p.spool)}
}

// ready is the /readyz state: a reason is "not_ready", "busy" or "draining".
func (p *Pipeline) ready() (bool, string) {
	switch {
	case p.active.isDraining():
		return false, "draining"
	case p.ctl.Policy() == nil || !p.ctl.Connected():
		return false, "not_ready"
	case p.active.used() >= p.cfg.MaxSessions:
		return false, "busy"
	}
	return true, ""
}
