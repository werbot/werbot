package dbproxy_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/dbproxy"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/platform/shellclass"
	"github.com/werbot/werbot/internal/platform/sqlclass"
)

// fakePDP decides by its rule and records every request.
type fakePDP struct {
	mu       sync.Mutex
	version  string
	profile  gateway.QueryProfile
	rule     func(gateway.AccessRequest) gateway.Decision // nil allows
	decided  []gateway.AccessRequest
	profiles int
}

func (p *fakePDP) Decide(r gateway.AccessRequest) gateway.Decision {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.decided = append(p.decided, r)
	if p.rule == nil {
		return gateway.Decision{Allow: true, Reasons: []string{"binding:b"}}
	}
	return p.rule(r)
}

func (p *fakePDP) Version() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.version
}

func (p *fakePDP) QueryProfile(gateway.AccessRequest) gateway.QueryProfile {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.profiles++
	return p.profile
}

// set changes the snapshot: a new version with rule.
func (p *fakePDP) set(rule func(gateway.AccessRequest) gateway.Decision) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.version, p.rule = p.version+"+", rule
}

func (p *fakePDP) decisions() []gateway.AccessRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]gateway.AccessRequest(nil), p.decided...)
}

// denyClasses denies every request whose classes contain one of cs, with
// obligation when set.
func denyClasses(message, obligation string, cs ...string) func(gateway.AccessRequest) gateway.Decision {
	return func(r gateway.AccessRequest) gateway.Decision {
		for _, c := range r.Classes {
			for _, d := range cs {
				if c == d && (obligation == "" || !r.ApprovalGranted) {
					d := gateway.Decision{Reasons: []string{"command:r1"}, Message: message}
					if obligation != "" {
						d.Obligations = []string{obligation}
					}
					return d
				}
			}
		}
		return gateway.Decision{Allow: true, Reasons: []string{"binding:b"}}
	}
}

// fakeDialect classifies by a table of texts; any other text is [read].
type fakeDialect struct {
	calls atomic.Int64
	res   map[string]sqlclass.Result
}

func (d *fakeDialect) Classify(text string, _ sqlclass.Options) sqlclass.Result {
	d.calls.Add(1)
	if r, ok := d.res[text]; ok {
		return r
	}
	return sqlclass.Result{Classes: []shellclass.Class{shellclass.Read}, Statements: 1}
}

// result is a classification of classes and tables.
func result(classes []shellclass.Class, tables, writes []string) sqlclass.Result {
	return sqlclass.Result{Classes: classes, Tables: tables, WriteTables: writes, Statements: 1}
}

// env is a Guard on fakes; typ is unique per test so the shared class cache starts empty.
type env struct {
	pdp      *fakePDP
	dialect  *fakeDialect
	sink     *spySink
	reg      *metrics.Registry
	denials  []gateway.ApprovalInput
	approve  func(ctx context.Context, in gateway.ApprovalInput, notify func(id, url string, timeout time.Duration) error) error
	approved []gateway.ApprovalInput
	limits   *gateway.AgentLimits
	secrets  []string
	typ      string
}

func newEnv(profile gateway.QueryProfile) *env {
	return &env{pdp: &fakePDP{version: "v1", profile: profile}, dialect: &fakeDialect{res: map[string]sqlclass.Result{}},
		sink: &spySink{}, reg: metrics.NewRegistry(), typ: "t" + uuid.NewString()[:8]}
}

func (e *env) hooks() gateway.SessionHooks {
	return gateway.SessionHooks{PDP: e.pdp, Limits: e.limits,
		RecordDenial: func(in gateway.ApprovalInput) { e.denials = append(e.denials, in) },
		Approve: func(ctx context.Context, in gateway.ApprovalInput, notify func(id, url string, timeout time.Duration) error) error {
			e.approved = append(e.approved, in)
			return e.approve(ctx, in, notify)
		}}
}

func (e *env) guard(t *testing.T) *dbproxy.Guard {
	t.Helper()
	g := dbproxy.NewGuard(&gateway.Session{Recording: e.sink}, e.hooks(),
		dbproxy.GuardConfig{Type: e.typ, Dialect: e.dialect, Options: sqlclass.Options{SecretTables: e.secrets}, Metrics: e.reg})
	t.Cleanup(g.Close)
	return g
}

// check is Check of op without an error.
func check(t *testing.T, g *dbproxy.Guard, op dbproxy.Op) dbproxy.Verdict {
	t.Helper()
	v, err := g.Check(t.Context(), op)
	if err != nil {
		t.Fatalf("check %q: %v", op.Text, err)
	}
	return v
}

func query(text string) dbproxy.Op { return dbproxy.Op{Kind: "query", Text: text, DB: "app"} }

var (
	cRead  = []shellclass.Class{shellclass.Read}
	cWrite = []shellclass.Class{shellclass.Write}
)
