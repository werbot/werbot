package gateway

import (
	"context"
	"net"
	"strconv"
	"sync"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/werbot/werbot/internal/connector"
)

func init() {
	// "rocheck": a database type whose connector checks agent_readonly
	// credentials through roFake; the config string is the endpoint host
	connector.Register(connector.Type{Name: "rocheck", NewConfig: func() proto.Message { return &wrapperspb.StringValue{} },
		Validate: func(proto.Message) error { return nil }, Actions: []string{"rocheck.connect", "rocheck.query"},
		Kinds: []connector.Kind{connector.KindStream},
		Endpoint: func(cfg proto.Message) connector.Endpoint {
			return connector.Endpoint{Host: cfg.(*wrapperspb.StringValue).GetValue(), Ports: []connector.PortRange{{Port: 5432}}}
		}})
	RegisterConnector(roConn{funcConn{typ: "rocheck", kinds: []Kind{KindStream}}})
}

// roConn is the "rocheck" connector: CheckReadonly answers through ro.
type roConn struct{ funcConn }

func (roConn) CheckReadonly(ctx context.Context, r Resource, primary, agent Secret, dial func(ctx context.Context, network, address string) (net.Conn, error)) (string, string) {
	return ro.check(ctx, r, primary, agent, dial)
}

// roFake records the checks of the "rocheck" type and answers with answer.
type roFake struct {
	mu      sync.Mutex
	answer  func(ctx context.Context) (status, detail string)
	running int
	peak    int
	checks  []roCall
}

type roCall struct {
	id             uuid.UUID
	primary, agent Secret
	dial           bool
}

// ro is the fake of the registered "rocheck" connector; useRO resets it.
var ro = &roFake{}

// useRO resets ro to answer ok, or with answer when set.
func useRO(t *testing.T, answer func(ctx context.Context) (string, string)) *roFake {
	t.Helper()
	if answer == nil {
		answer = func(context.Context) (string, string) { return ReadonlyOK, "" }
	}
	ro.mu.Lock()
	defer ro.mu.Unlock()
	ro.answer, ro.running, ro.peak, ro.checks = answer, 0, 0, nil
	return ro
}

func (f *roFake) check(ctx context.Context, r Resource, primary, agent Secret, dial func(context.Context, string, string) (net.Conn, error)) (string, string) {
	f.mu.Lock()
	f.running++
	f.peak = max(f.peak, f.running)
	f.checks = append(f.checks, roCall{id: r.ID, primary: primary, agent: agent, dial: dial != nil})
	answer := f.answer
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.running--
		f.mu.Unlock()
	}()
	return answer(ctx)
}

func (f *roFake) calls() []roCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]roCall(nil), f.checks...)
}

func (f *roFake) maxRunning() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.peak
}

// probeCreds is the GetProbeCredential of a test: it answers secret and
// version, or err, and records the calls.
type probeCreds struct {
	mu      sync.Mutex
	secret  Secret
	version int64
	err     error
	calls   []credAsk
}

type credAsk struct {
	id     uuid.UUID
	config int64
}

func (c *probeCreds) get(_ context.Context, id uuid.UUID, config int64) (Secret, int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, credAsk{id, config})
	return c.secret, c.version, c.err
}

func (c *probeCreds) asked() []credAsk {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]credAsk(nil), c.calls...)
}

func (c *probeCreds) set(fn func(*probeCreds)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fn(c)
}

// rores is a "rocheck" resource uid(n) on host 10.0.1.n with the check status
// and configuration version.
func rores(n int, status string, config int64) Resource {
	return Resource{ID: uid(n), Name: "db" + strconv.Itoa(n), Type: "rocheck", Config: []byte(strconv.Quote("10.0.1." + strconv.Itoa(n))),
		AgentReadonly: status, ConfigVersion: config}
}
