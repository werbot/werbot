package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"runtime/debug"
	"slices"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/gateway/recorder"
	"github.com/werbot/werbot/internal/platform/metrics"
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
	Target   string // KindStream, KindDatagram: host:port
	ClientIP netip.Addr
	Window   Window // KindTerminal: initial size for the recorder
}

// control is the control plane of the pipeline; *tunnelClient implements it.
type control interface {
	AuthenticateUser(ctx context.Context, fingerprint string, ip netip.Addr) (User, error)
	StartSession(ctx context.Context, userID, resourceID uuid.UUID, ip netip.Addr, kind Kind, target string) (uuid.UUID, Secret, error)
	PinHostKey(ctx context.Context, resourceID uuid.UUID, alg string, key []byte) error
	Policy() *Policy
	PolicyChanged() <-chan struct{} // receives after a new Policy is stored
	Connected() bool
}

// endCause cancels a session context; its text is the end reason.
type endCause string

func (c endCause) Error() string { return string(c) }

const (
	causeShutdown endCause = "gateway_shutdown"
	causeInternal endCause = "internal_error"
)

// errServed cancels a session context once Serve returned: a later cancel
// (revoke, idle) no longer changes the end reason.
var errServed = errors.New("session served")

// Pipeline opens sessions against the control plane and runs them on connectors.
type Pipeline struct {
	ctl    control
	cfg    Config
	active *activeSessions
	spool  *recorder.Spool // session ends and recordings on their way to the server; set by serve
	log    *slog.Logger

	busy, notReady, spoolFull, panics *metrics.Counter
	meters                            meters
	denials                           *denials
	beats                             atomic.Uint64 // heartbeats built
	guard                             localGuard
	dial                              func(ctx context.Context, network, address string) (net.Conn, error)

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
	p := &Pipeline{
		ctl: ctl, cfg: cfg, log: logger, stop: stop, stopNow: stopNow,
		active:       newActiveSessions(reg.Gauge("werbot_gateway_sessions_active", "Sessions holding a MaxSessions slot."), logger),
		busy:         reg.Counter("werbot_gateway_sessions_rejected_total", rejectedHelp, "reason", "busy"),
		notReady:     reg.Counter("werbot_gateway_sessions_rejected_total", rejectedHelp, "reason", "not_ready"),
		spoolFull:    reg.Counter("werbot_gateway_sessions_rejected_total", rejectedHelp, "reason", "spool_full"),
		panics:       reg.Counter("werbot_gateway_panics_total", "Connector panics recovered by the pipeline."),
		meters:       newMeters(reg),
		denials:      newDenials(reg),
		guard:        localGuard{deny: cfg.DenyLocalTargets, hosts: cfg.DenyHosts},
		flushTimeout: 10 * time.Second,
	}
	p.dial = p.guard.dialer()
	return p
}

// Open checks the request in contract order and starts the session on the server.
// Any error frees the slot and, unless it is ErrNotReady, goes to the next
// heartbeat as a denial. A spool at SpoolMax, or one that cannot write, refuses
// the kinds the resource type records: recording is mandatory for them.
func (p *Pipeline) Open(ctx context.Context, r OpenRequest) (*Session, error) {
	if err := p.active.reserve(p.cfg.MaxSessions); err != nil {
		p.deny(r, uuid.Nil, err)
		return nil, p.reject(err)
	}
	s, user, err := p.open(ctx, r)
	if err != nil {
		p.active.release()
		p.deny(r, user, err)
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
	case errors.Is(err, ErrSpoolMax):
		p.spoolFull.Inc()
	}
	return err
}

// open returns the user it authenticated, also with an error, for the denial.
func (p *Pipeline) open(ctx context.Context, r OpenRequest) (*Session, uuid.UUID, error) {
	pol := p.ctl.Policy()
	if pol == nil || !p.ctl.Connected() {
		return nil, uuid.Nil, ErrNotReady
	}
	u, err := p.ctl.AuthenticateUser(ctx, r.Identity.Value, r.ClientIP)
	if err != nil {
		return nil, uuid.Nil, controlErr(err)
	}
	res, ok := pol.Resource(r.Resource)
	if !ok {
		return nil, u.ID, ErrDenied
	}
	req := AccessRequest{User: u, Resource: res, IP: r.ClientIP}
	var host string
	if r.Kind == KindStream || r.Kind == KindDatagram {
		if host, req.Port, err = allowTarget(res, r.Target); err != nil {
			return nil, u.ID, err
		}
	}
	if allow, _ := pol.Authorize(req); !allow {
		return nil, u.ID, ErrDenied
	}
	// after the PDP: only an allowed user learns that the spool is full
	typ, _ := connector.Lookup(res.Type)
	recorded := typ.Records(r.Kind)
	if recorded && (p.spool.Total() >= p.cfg.SpoolMax || !p.spool.Writable()) {
		return nil, u.ID, ErrSpoolMax
	}
	c, ok := LookupConnector(res.Type)
	if !ok {
		return nil, u.ID, ErrUnsupportedType
	}
	if !slices.Contains(c.Kinds(), r.Kind) {
		return nil, u.ID, ErrUnsupportedKind
	}
	if _, dials := c.(TargetDialer); dials {
		if err := p.guard.allowHost(ctx, dialHost(res, host)); err != nil {
			return nil, u.ID, err
		}
	}
	id, secret, err := p.ctl.StartSession(ctx, u.ID, res.ID, r.ClientIP, r.Kind, r.Target)
	if err != nil {
		return nil, u.ID, controlErr(err)
	}
	rec := NopRecording(id, 0, 0)
	if recorded {
		rec = p.cfg.Recording(id, r.Window.Cols, r.Window.Rows)
	}
	t := &tracked{req: req, kind: r.Kind, host: host, version: pol.Version(), started: time.Now()}
	t.idle, t.max = limits(res, p.cfg)
	p.active.add(id, t) // reported in heartbeats from now on, so the server keeps the session
	return &Session{
		ID: id, Kind: r.Kind, Front: r.Front, User: u, Resource: res, Secret: secret, ClientIP: r.ClientIP, Target: r.Target,
		PinKey:    func(alg string, key []byte) error { return p.ctl.PinHostKey(p.stop, res.ID, alg, key) },
		Dial:      p.dial,
		Recording: rec,
	}, u.ID, nil
}

// controlErr maps a control plane error: an outage is ErrNotReady, anything else ErrDenied.
func controlErr(err error) error {
	if unreachable(err) {
		return fmt.Errorf("%w: %w", ErrNotReady, err)
	}
	return fmt.Errorf("%w: %w", ErrDenied, err)
}

// Run serves s on c until it ends, then spools the session end with its counters,
// closes the recording and frees the slot. A connector panic ends only this
// session. A terminal ended by the sweeper is told why.
func (p *Pipeline) Run(ctx context.Context, s *Session, c Connector) error {
	sctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	meter{p.active.bind(s.ID, cancel), p.meters}.wrap(s)
	panicked, err := p.serve(sctx, s, c, cancel)
	cancel(errServed)
	reason := endReason(sctx, err)
	if panicked { // also after a client cancel or drain
		reason = string(causeInternal)
	}
	if reason == "error" {
		p.log.Warn("session: connector failed", "session_id", s.ID, "type", c.Type(), "error", s.Secret.Redact(err.Error()))
	}
	if text := closeText[reason]; text != "" && s.Terminal != nil {
		say(s.Terminal, text) // after Serve: never concurrent with the connector's writes
	}
	p.finish(ctx, s, reason)
	return err
}

// closeText is what a terminal reads when the sweeper ends it.
var closeText = map[string]string{
	string(causeIdle): "Session closed: idle timeout",
	string(causeMax):  "Session closed: max duration reached",
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
	var final *gatewayv1.SessionCounters
	if c := p.active.countersOf(s.ID); c != nil {
		final = countersProto(s.ID, c.load())
	}
	if err := p.spool.End(s.ID, reason, final); err != nil {
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
// a client cancel is "closed", a connector error "error". errServed is no cause.
func endReason(ctx context.Context, err error) string {
	cause := context.Cause(ctx)
	var c endCause
	if errors.As(cause, &c) {
		return string(c)
	}
	if err == nil || (ctx.Err() != nil && !errors.Is(cause, errServed)) {
		return "closed"
	}
	return "error"
}
