package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/gateway/recorder"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/platform/sshca"
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

// Identity is how the front identified the user: "ssh-key" or "ssh-cert" and
// the SHA256 fingerprint of the key or certificate; Cert is the werbot
// identity of a certificate the front verified.
type Identity struct {
	Method, Value string
	Cert          *sshca.Identity
}

// OpenRequest is what a front asks the pipeline to open.
type OpenRequest struct {
	Front    string
	Kind     Kind
	Identity Identity
	Resource string // resource name
	Target   string // KindStream, KindDatagram: host:port
	ClientIP netip.Addr
	Window   Window    // KindTerminal: initial size for the recorder
	Command  string    // KindExec, KindTerminal with a command: the command to run
	Notice   io.Writer // the user's side channel: stderr of an exec, the PTY of a terminal
	// Handshake is the user the front authenticated for Identity; nil: none.
	Handshake *Handshake
	ToolCall  uuid.UUID // an agent's tool call (sshca.EnvToolCall); uuid.Nil: none
	MFA       MFAStep
	Transport string // "tcp" | "ws" | "internal" (the web terminal); "": tcp
	// RemoteForward: KindStream from tcpip-forward, Target "<bind_host>:<bind_port>".
	RemoteForward bool
}

// control is the control plane of the pipeline; *tunnelClient implements it.
type control interface {
	AuthenticateUser(ctx context.Context, id Identity, ip netip.Addr) (User, error)
	StartSession(ctx context.Context, userID, resourceID uuid.UUID, ip netip.Addr, kind Kind, target, command string, classes, paths []string, args startArgs) (uuid.UUID, Secret, error)
	RecordDenial(ctx context.Context, runID uuid.UUID, resource, command string, classes, reasons []string) (paused bool, err error)
	PinHostKey(ctx context.Context, resourceID uuid.UUID, alg string, key []byte) error
	RequestApproval(ctx context.Context, r approvalRequest) (approvalTicket, error)
	GetApproval(ctx context.Context, id string) (status string, err error)
	SubscribeApproval(id string) (decided <-chan struct{}, unsubscribe func())
	Policy() *Policy
	PolicyChanged() <-chan struct{} // receives after a new Policy is stored
	Connected() bool
	DisconnectedSince() time.Time // zero while Connected; else when the tunnel was lost
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
	health                            *prober
	dial                              func(ctx context.Context, network, address string) (net.Conn, error)
	now                               func() time.Time
	dropped                           *metrics.Counter // live view events observers lost: they fell behind

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
		now:          time.Now,
		dropped:      reg.Counter("werbot_gateway_observer_drops_total", "Live view events an observer lost: it fell behind."),
	}
	p.dial = p.guard.dialer()
	p.health = newProber(ctl.Policy, tcpProbe(p.dial), reg)
	return p
}

// Open checks the request in contract order and starts the session on the server.
// Any error frees the slot and, unless it is ErrNotReady, is reported as a denial
// (p.deny). A spool at SpoolMax, or one that cannot write, refuses the kinds the
// resource type records: recording is mandatory for them.
func (p *Pipeline) Open(ctx context.Context, r OpenRequest) (*Session, error) {
	if err := p.active.reserve(p.cfg.MaxSessions); err != nil {
		p.deny(ctx, r, AccessRequest{}, err)
		return nil, p.reject(err)
	}
	s, req, err := p.open(ctx, r)
	if err != nil {
		p.active.release()
		p.deny(ctx, r, req, err)
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

// open returns the request it decided, also with an error, for the denial. After
// an approval wait that is the recheck's: the session runs on its user and resource.
func (p *Pipeline) open(ctx context.Context, r OpenRequest) (*Session, AccessRequest, error) {
	pol, req, host, err := p.decide(ctx, r)
	if err != nil {
		return nil, req, err
	}
	typ, _ := connector.Lookup(req.Resource.Type)
	recorded := typ.Records(r.Kind)
	if err := p.servable(ctx, r.Kind, req.Resource, host, recorded); err != nil {
		return nil, req, err
	}
	if err := p.shellBeforeCommand(pol, req, r, typ); err != nil {
		return nil, req, err
	}
	if pol, req, err = p.authorizeCommand(ctx, pol, withExecAction(req, r, typ), commandInput{Text: r.Command, Cwd: "~"}, r.Notice); err != nil {
		return nil, req, err
	}
	if req, err = p.authorizeShell(pol, req, r, typ); err != nil {
		return nil, req, err
	}
	u, res := req.User, req.Resource
	id, secret, err := p.ctl.StartSession(ctx, u.ID, res.ID, r.ClientIP, r.Kind, r.Target, req.Command, req.Classes, req.Paths, startOf(u, r))
	if err != nil {
		return nil, req, startErr(err)
	}
	rec := p.recording(id, r, recorded)
	t := &tracked{req: req, kind: r.Kind, host: host, version: pol.Version(), started: time.Now(), rec: rec}
	t.idle, t.max = limits(res, p.cfg)
	p.active.add(id, t) // reported in heartbeats from now on, so the server keeps the session
	s := &Session{
		ID: id, Kind: r.Kind, Front: r.Front, User: u, Resource: res, Secret: secret, ClientIP: r.ClientIP, Target: r.Target,
		Command: r.Command, Classes: req.Classes, Paths: req.Paths, Console: req.Console,
		PinKey:    func(alg string, key []byte) error { return p.pinKey(res, alg, key) },
		Dial:      p.dial,
		Recording: rec,
		Metrics:   p.cfg.Metrics,
	}
	s.CheckCommand = p.commandChecker(req, r, s)
	return s, req, nil
}

// Run serves s on c until it ends, then spools the session end with its counters,
// closes the recording and frees the slot. A connector panic ends only this
// session. A terminal ended by the sweeper is told why.
func (p *Pipeline) Run(ctx context.Context, s *Session, c Connector) error {
	sctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	meter{p.active.bind(s.ID, cancel), p.meters}.wrap(s)
	s.trackOutcome()
	panicked, err := p.serve(sctx, s, c, cancel)
	cancel(errServed)
	reason := s.ended(sctx, err)
	if panicked { // also after a client cancel or drain
		reason = string(causeInternal)
	}
	if reason == "error" && err != nil { // a lost web stream is "error" without a connector error
		p.log.Warn("session: connector failed", "session_id", s.ID, "type", c.Type(), "error", s.Secret.Redact(err.Error()))
	}
	if text := closeText[reason]; text != "" && s.Terminal != nil {
		say(s.Terminal, text) // after Serve: never concurrent with the connector's writes
	}
	p.finish(ctx, s, reason)
	return err
}

// errConnectorPanic is the Run error of a session whose connector panicked.
var errConnectorPanic = errors.New("connector panicked")

func (p *Pipeline) serve(ctx context.Context, s *Session, c Connector, cancel context.CancelCauseFunc) (panicked bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			cancel(causeInternal)
			p.panics.Inc()
			p.log.Error("session: connector panic", "session_id", s.ID, "type", c.Type(),
				"panic", s.Secret.Redact(fmt.Sprint(r)), "stack", s.Secret.Redact(string(debug.Stack())))
			panicked, err = true, fmt.Errorf("%w: %s", errConnectorPanic, c.Type())
		}
	}()
	defer maskExec(s, p.ctl.Policy())() // flushes right after Serve, before Run returns
	if s.Console {
		return false, c.(ConsoleServer).ServeConsole(ctx, s) // Open checked it: never a raw shell
	}
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
	if err := p.spool.End(s.ID, reason, final, s.result()); err != nil {
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
