package gateway

import (
	"context"
	"io"
	"net/netip"
	"strings"
	"testing"

	"buf.build/go/protovalidate"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/connector"
)

func init() {
	// plainstream is an unrecorded stream and datagram type, like tcp: its config is
	// the target host, its ports 5432-5433
	connector.Register(connector.Type{Name: "plainstream", NewConfig: func() proto.Message { return &wrapperspb.StringValue{} },
		Validate: func(proto.Message) error { return nil }, Actions: []string{"plainstream.connect"},
		Kinds: []connector.Kind{connector.KindStream, connector.KindDatagram},
		Endpoint: func(m proto.Message) connector.Endpoint {
			return connector.Endpoint{Host: m.(*wrapperspb.StringValue).GetValue(), Ports: []connector.PortRange{{Port: 5432, EndPort: 5433}}}
		}})
	RegisterConnector(dialingConn{funcConn{typ: "plainstream", kinds: []Kind{KindStream, KindDatagram}, serve: echo}})
}

// dialingConn dials its target through Session.Dial, like tcp.
type dialingConn struct{ funcConn }

func (dialingConn) DialsTarget() {}

// echo copies the stream, or each datagram, back until the client closes it or ctx ends.
func echo(ctx context.Context, s *Session) error {
	if s.Datagram != nil {
		return echoDatagrams(ctx, s.Datagram)
	}
	stop := context.AfterFunc(ctx, func() { _ = s.Stream.Close() })
	defer stop()
	_, err := io.Copy(s.Stream, s.Stream)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// plainPolicies permits u1 everything but port 5433 of "pg".
const plainPolicies = pipePolicies + `
forbid (principal, action, resource == Werbot::Resource::"rpg") when { context.port == 5433 };`

// plainSnapshot is the pipeEnv snapshot plus "pg" (plainstream on 10.0.0.5), with
// version v and edit applied before it compiles.
func (e *pipeEnv) plainSnapshot(t *testing.T, policies, v string, edit func(*gatewayv1.Snapshot)) *Policy {
	t.Helper()
	if _, ok := e.m["rpg"]; !ok {
		e.m["rpg"] = uuid.New()
	}
	s := testSnapshot(t, e.m, policies, nil, res{"rfake", "fake", "fake"}, res{"rblocker", "blocker", "blocker"}, res{"rpg", "pg", "plainstream"})
	s.Version = v
	s.Resources[2].ConfigJson = []byte(`"10.0.0.5"`)
	if edit != nil {
		edit(s)
	}
	p, err := CompilePolicy(s)
	require.NoError(t, err)
	return p
}

func newPlainEnv(t *testing.T) *pipeEnv {
	t.Helper()
	e := newPipeEnv(t, 10)
	e.f.setPolicy(e.plainSnapshot(t, plainPolicies, "v1", nil))
	return e
}

func (e *pipeEnv) target(kind Kind, target string) OpenRequest {
	r := e.req("pg", kind)
	r.Target = target
	return r
}

// requireValid checks a heartbeat against its wire rules, as the server does.
func requireValid(t *testing.T, hb *gatewayv1.Heartbeat) {
	t.Helper()
	require.NoError(t, protovalidate.Validate(hb))
}

func TestOpenTargetNotAllowed(t *testing.T) {
	e := newPlainEnv(t)
	for _, target := range []string{"pg:5434", "10.9.9.9:5432", "pg:0", "pg", ":5432", "pg:99999", "pg:http"} {
		_, err := e.p.Open(t.Context(), e.target(KindStream, target))
		require.ErrorIs(t, err, ErrTargetNotAllowed, target)
	}
	_, err := e.p.Open(t.Context(), e.target(KindDatagram, "10.9.9.9:5432"))
	require.ErrorIs(t, err, ErrTargetNotAllowed)
	_, err = e.p.Open(t.Context(), e.target(KindStream, "pg:5433"))
	require.ErrorIs(t, err, ErrDenied, "the target port reaches the PDP")
	e.f.with(func(f *fakeControl) { require.Zero(t, f.startCalls) })
	require.Zero(t, e.p.active.used())

	hb := e.p.heartbeat()
	requireValid(t, hb)
	require.Len(t, hb.GetDenials(), 9)
	d := hb.GetDenials()[1]
	require.Equal(t, "target_not_allowed", d.GetReason())
	require.Equal(t, e.m["u1"].String(), d.GetUserId())
	require.Equal(t, "SHA256:key", d.GetKeyFingerprint())
	require.Equal(t, "pg", d.GetResource())
	require.Equal(t, "stream", d.GetKind())
	require.Equal(t, "10.9.9.9:5432", d.GetTarget())
	require.Equal(t, "192.0.2.7", d.GetClientIp())
	require.NotNil(t, d.GetAt())
	require.Equal(t, "denied", hb.GetDenials()[8].GetReason())
	require.Empty(t, e.p.heartbeat().GetDenials(), "a heartbeat takes the denials")

	for _, target := range []string{"pg:5432", "PG:5432", "10.0.0.5:5432"} {
		s, err := e.p.Open(t.Context(), e.target(KindStream, target))
		require.NoError(t, err, target)
		require.Equal(t, target, s.Target)
		require.NotNil(t, s.Dial)
	}
	s, err := e.p.Open(t.Context(), e.target(KindDatagram, "pg:5432"))
	require.NoError(t, err)
	require.Equal(t, KindDatagram, s.Kind)
}

func TestOpenNotRecordedKind(t *testing.T) {
	e := newPlainEnv(t)
	e.open(t, "fake", KindExec) // recorded: the factory runs
	s, err := e.p.Open(t.Context(), e.target(KindStream, "pg:5432"))
	require.NoError(t, err)
	require.Equal(t, nopSink{}, s.Recording)
	e.f.with(func(f *fakeControl) { require.Len(t, f.recs, 1, "no recorder for an unrecorded kind") })

	e.p.cfg.SpoolMax = 100
	e.f.with(func(f *fakeControl) { f.upErr = errDown })
	require.NoError(t, e.p.spool.Append(uuid.New(), 0, make([]byte, 32), make([]byte, 100)))
	_, err = e.p.Open(t.Context(), e.target(KindStream, "pg:5432"))
	require.NoError(t, err, "a full spool does not stop an unrecorded kind")
	_, err = e.p.Open(t.Context(), e.req("fake", KindTerminal))
	require.ErrorIs(t, err, ErrSpoolMax)
	require.Contains(t, e.metrics(t), `werbot_gateway_sessions_rejected_total{reason="spool_full"} 1`)
	require.Equal(t, "spool_full", e.p.heartbeat().GetDenials()[0].GetReason())
}

func TestDenialsCapped(t *testing.T) {
	e := newPipeEnv(t, 10)
	for range 150 {
		_, err := e.p.Open(t.Context(), e.req("missing", KindTerminal))
		require.ErrorIs(t, err, ErrDenied)
	}
	hb := e.p.heartbeat()
	requireValid(t, hb)
	require.Len(t, hb.GetDenials(), 100)
	require.Equal(t, e.m["u1"].String(), hb.GetDenials()[0].GetUserId(), "a missing resource is refused after AuthenticateUser")
	require.Contains(t, e.metrics(t), "werbot_gateway_denials_dropped_total 50")
	require.Empty(t, e.p.heartbeat().GetDenials())

	e.f.with(func(f *fakeControl) { f.connected = false })
	_, err := e.p.Open(t.Context(), e.req("fake", KindTerminal))
	require.ErrorIs(t, err, ErrNotReady)
	require.Empty(t, e.p.heartbeat().GetDenials(), "not_ready is not reported")
}

// Every string of a denial fits its wire bound, so one long or odd value cannot
// close the tunnel stream.
func TestDenialFitsWire(t *testing.T) {
	long := strings.Repeat("é", 400)
	d := newDenial("denied", uuid.Nil, long, long, Kind(long), long, netip.MustParseAddr("fe80::1%eth0"))
	requireValid(t, &gatewayv1.Heartbeat{Denials: []*gatewayv1.Denial{d}})
	require.Equal(t, "fe80::1", d.GetClientIp())
	require.Len(t, d.GetResource(), 62, "cut before a split character")
	d = newDenial("denied", uuid.Nil, "", "", "", "", netip.MustParseAddr("::ffff:192.0.2.7"))
	require.Equal(t, "192.0.2.7", d.GetClientIp())
}

// With DenyLocalTargets, Open refuses a dialing connector whose endpoint host is
// refused before StartSession, and reports it.
func TestOpenDeniesLocalTarget(t *testing.T) {
	e := newPlainEnv(t)
	e.p.guard = localGuard{deny: true, hosts: []string{"10.0.0.5"}} // "pg" is the database host
	_, err := e.p.Open(t.Context(), e.target(KindStream, "pg:5432"))
	require.ErrorIs(t, err, ErrTargetNotAllowed)
	e.f.with(func(f *fakeControl) { require.Zero(t, f.startCalls) })
	d := e.p.heartbeat().GetDenials()
	require.Len(t, d, 1)
	require.Equal(t, "target_not_allowed", d[0].GetReason())
	require.Equal(t, e.m["u1"].String(), d[0].GetUserId())

	e.p.guard.hosts = nil
	_, err = e.p.Open(t.Context(), e.target(KindStream, "pg:5432"))
	require.NoError(t, err)
	e.open(t, "fake", KindTerminal) // not a dialing connector: not checked
}
