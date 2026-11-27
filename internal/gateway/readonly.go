package gateway

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/metrics"
)

// Results of a ReadonlyChecker.
const (
	ReadonlyOK       = "ok"
	ReadonlyHasWrite = "has_write"
	ReadonlyError    = "error"
)

const (
	readonlyRecheck   = 10 * time.Minute // a checked status is checked again after this
	readonlyTimeout   = 30 * time.Second // one check, the credential call included
	readonlyParallel  = 2                // concurrent checks, beside the probeParallel probes
	maxReadonlyDetail = 512              // max_bytes of ResourceHealth.agent_readonly_detail
)

// ReadonlyChecker is an optional Connector interface of database types: the
// health prober calls CheckReadonly with the agent_readonly credential (agent;
// primary is empty until a type needs it) and the pipeline's guarded dialer.
// status is ok only when the credential can write nothing it can reach,
// has_write with up to five findings in detail, else error; detail holds no
// credential.
type ReadonlyChecker interface {
	CheckReadonly(ctx context.Context, r Resource, primary, agent Secret,
		dial func(ctx context.Context, network, address string) (net.Conn, error)) (status, detail string)
}

// readonlyCheck is the result of one check: its status and detail at the
// check version of the credential, and when it ended.
type readonlyCheck struct {
	status, detail string
	version        int64
	at             time.Time
}

// readonlyChecks runs the agent_readonly checks of the resources this
// instance probes, in the background, at most readonlyParallel at a time, and
// keeps each result until the heartbeat takes it.
type readonlyChecks struct {
	cred   func(ctx context.Context, resourceID uuid.UUID, configVersion int64) (Secret, int64, error)
	dial   func(ctx context.Context, network, address string) (net.Conn, error)
	counts map[string]*metrics.Counter
	sem    chan struct{}

	mu      sync.Mutex
	running map[uuid.UUID]bool
	last    map[uuid.UUID]readonlyCheck // the latest check of each resource
	results map[uuid.UUID]readonlyCheck // the checks the heartbeat has not taken
}

func newReadonlyChecks(cred func(context.Context, uuid.UUID, int64) (Secret, int64, error),
	dial func(context.Context, string, string) (net.Conn, error), reg *metrics.Registry,
) *readonlyChecks {
	const help = "Checks of agent_readonly credentials by result."
	c := &readonlyChecks{cred: cred, dial: dial, counts: map[string]*metrics.Counter{}, sem: make(chan struct{}, readonlyParallel),
		running: map[uuid.UUID]bool{}, last: map[uuid.UUID]readonlyCheck{}, results: map[uuid.UUID]readonlyCheck{}}
	for _, s := range []string{ReadonlyOK, ReadonlyHasWrite, ReadonlyError} {
		c.counts[s] = reg.Counter("werbot_gateway_agent_readonly_checks_total", help, "result", s)
	}
	return c
}

// readonlyCheckerOf is the ReadonlyChecker of the connector of typ; nil when it has none.
func readonlyCheckerOf(typ string) ReadonlyChecker {
	c, _ := LookupConnector(typ)
	rc, _ := c.(ReadonlyChecker)
	return rc
}

// start checks r in the background when a check is due: r has an
// agent_readonly credential, its type a ReadonlyChecker, and r is pending or
// its latest check is readonlyRecheck old. ctx ends the check.
func (c *readonlyChecks) start(ctx context.Context, r Resource) {
	if c == nil || r.AgentReadonly == "" || r.AgentReadonly == "none" {
		return
	}
	rc := readonlyCheckerOf(r.Type)
	if rc == nil || !c.reserve(r) {
		return
	}
	go c.run(ctx, r, rc)
}

// reserve marks a due check of r as running.
func (c *readonlyChecks) reserve(r Resource) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	last, seen := c.last[r.ID]
	if c.running[r.ID] || (seen && r.AgentReadonly != "pending" && time.Since(last.at) < readonlyRecheck) {
		return false
	}
	c.running[r.ID] = true
	return true
}

// run gets the credential for the configuration of the snapshot and checks
// it, unless this check version was checked within readonlyRecheck. A
// credential error is retried on the next pass; a stopped ctx reports nothing.
func (c *readonlyChecks) run(ctx context.Context, r Resource, rc ReadonlyChecker) {
	defer c.release(r.ID)
	select {
	case c.sem <- struct{}{}:
	case <-ctx.Done():
		return
	}
	defer func() { <-c.sem }()
	cctx, cancel := context.WithTimeout(ctx, readonlyTimeout)
	defer cancel()
	agent, version, err := c.cred(cctx, r.ID, r.ConfigVersion)
	if err != nil {
		slog.WarnContext(ctx, "gateway: agent_readonly check skipped", "resource_id", r.ID, "error", err)
		return
	}
	if c.checked(r.ID, version) {
		return
	}
	status, detail := rc.CheckReadonly(cctx, r, Secret{}, agent, c.dial)
	if ctx.Err() != nil {
		return
	}
	if status != ReadonlyOK && status != ReadonlyHasWrite { // any other answer fails closed
		status = ReadonlyError
	}
	c.record(r.ID, readonlyCheck{status: status, detail: TruncateUTF8(agent.Redact(detail), maxReadonlyDetail), version: version, at: time.Now()})
}

// checked reports whether version of resource id was checked within readonlyRecheck.
func (c *readonlyChecks) checked(id uuid.UUID, version int64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	last, seen := c.last[id]
	return seen && last.version == version && time.Since(last.at) < readonlyRecheck
}

func (c *readonlyChecks) record(id uuid.UUID, res readonlyCheck) {
	c.counts[res.status].Inc()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.last[id], c.results[id] = res, res
}

func (c *readonlyChecks) release(id uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.running, id)
}

// attach moves the check result of resource id, if any, into h.
func (c *readonlyChecks) attach(id uuid.UUID, h *gatewayv1.ResourceHealth) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if res, ok := c.results[id]; ok {
		h.AgentReadonlyStatus, h.AgentReadonlyDetail, h.AgentReadonlyVersion = res.status, res.detail, res.version
		delete(c.results, id)
	}
}
