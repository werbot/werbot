package gateway

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/werbot/werbot/internal/connector"
)

func init() {
	// rfwd is an ssh-like type whose config is its remote_forwarding flag;
	// rfwdbare permits remote forwards but declares no remote_forward action.
	for _, n := range []string{"rfwd", "rfwdbare"} {
		actions := []string{n + ".connect"}
		if n == "rfwd" {
			actions = append(actions, n+".remote_forward")
		}
		connector.Register(connector.Type{Name: n, NewConfig: func() proto.Message { return &wrapperspb.BoolValue{} },
			Validate: func(proto.Message) error { return nil }, Actions: actions, Kinds: []connector.Kind{connector.KindStream},
			Endpoint: func(m proto.Message) connector.Endpoint {
				return connector.Endpoint{Ports: []connector.PortRange{{Port: 22}}, NoTarget: true, RemoteForward: m.(*wrapperspb.BoolValue).GetValue()}
			}})
		RegisterConnector(funcConn{typ: n, kinds: []Kind{KindStream}, serve: fwdTarget})
	}
}

// Bind ports the fake target treats specially.
const (
	fwdRefused = 1001 // refuses tcpip-forward
	fwdSilent  = 1002 // never listens
)

// fwdTargets are the running fake forwards by bind port.
var fwdTargets sync.Map

// fwdTarget serves a remote forward like a target: ready at once, then the
// test opens connections through fwdTargets until ctx ends.
func fwdTarget(ctx context.Context, s *Session) error {
	rf := s.RemoteForward
	switch {
	case rf == nil:
		return errors.New("not a remote forward")
	case rf.BindPort == fwdRefused:
		return errors.New("tcpip-forward refused")
	case rf.BindPort == fwdSilent:
		<-ctx.Done()
		return nil
	}
	fwdTargets.Store(rf.BindPort, rf)
	defer fwdTargets.Delete(rf.BindPort)
	rf.Ready()
	<-ctx.Done()
	return nil
}

// fwdPolicy is a snapshot of policies over "fwd" (rfwd with the flag on),
// "bare" (rfwdbare, flag on) and "blocker", of version v.
func fwdPolicy(t *testing.T, m ids, v, policies string, on bool) *Policy {
	t.Helper()
	for _, n := range []string{"rfwd", "rbare", "rblocker"} {
		if _, ok := m[n]; !ok {
			m[n] = uuid.New()
		}
	}
	s := testSnapshot(t, m, policies, nil, res{"rfwd", "fwd", "rfwd"}, res{"rbare", "bare", "rfwdbare"}, res{"rblocker", "blocker", "blocker"})
	s.Version = v
	s.Resources[0].ConfigJson = []byte(strconv.FormatBool(on))
	s.Resources[1].ConfigJson = []byte("true")
	p, err := CompilePolicy(s)
	require.NoError(t, err)
	return p
}

// fwdReq is a remote forward request on resource for bind host:port.
func (e *pipeEnv) fwdReq(resource, host string, port uint16) OpenRequest {
	r := e.req(resource, KindStream)
	r.Target, r.RemoteForward = net.JoinHostPort(host, strconv.Itoa(int(port))), true
	return r
}

func TestOpenRemoteForward(t *testing.T) {
	e := newPipeEnv(t, 10)
	e.f.setPolicy(fwdPolicy(t, e.m, "v1", pipePolicies, false))
	_, err := e.p.Open(t.Context(), e.fwdReq("fwd", "localhost", 8080))
	require.ErrorIs(t, err, ErrTargetNotAllowed, "remote_forwarding off")
	require.Equal(t, "target_not_allowed", e.p.heartbeat().GetDenials()[0].GetReason())
	e.f.with(func(f *fakeControl) { require.Zero(t, f.startCalls) })

	e.f.setPolicy(fwdPolicy(t, e.m, "v2", pipePolicies, true))
	s, err := e.p.Open(t.Context(), e.fwdReq("fwd", "localhost", 8080))
	require.NoError(t, err)
	require.Equal(t, KindStream, s.Kind)
	require.Equal(t, "localhost:8080", s.Target)
	require.Equal(t, nopSink{}, s.Recording, "a stream of the type is not recorded")
	e.p.abort(s)

	for _, target := range []string{"localhost:0", "localhost", "localhost:99999"} {
		r := e.fwdReq("fwd", "", 1)
		r.Target = target
		_, err = e.p.Open(t.Context(), r)
		require.ErrorIs(t, err, ErrTargetNotAllowed, target)
	}
	_, err = e.p.Open(t.Context(), e.fwdReq("bare", "localhost", 8080))
	require.ErrorIs(t, err, ErrUnsupportedKind, "the type has no remote_forward action")

	e.f.setPolicy(fwdPolicy(t, e.m, "v3", pipePolicies+`
forbid (principal, action == Werbot::Action::"rfwd.remote_forward", resource);`, true))
	_, err = e.p.Open(t.Context(), e.fwdReq("fwd", "localhost", 8080))
	require.ErrorIs(t, err, ErrDenied, "a forbid on the action")
	s, err = e.p.Open(t.Context(), e.req("fwd", KindStream))
	require.ErrorIs(t, err, ErrTargetNotAllowed, "a plain stream is still no remote forward")
	require.Nil(t, s)

	// A binding limited to port 22 bounds the bind port.
	e.f.setPolicy(fwdPolicy(t, e.m, "v4", `permit (principal, action, resource) when { context.port == 22 };`, true))
	_, err = e.p.Open(t.Context(), e.fwdReq("fwd", "localhost", 8080))
	require.ErrorIs(t, err, ErrDenied)
	s, err = e.p.Open(t.Context(), e.fwdReq("fwd", "localhost", 22))
	require.NoError(t, err)
	e.p.abort(s)
}

// The connect decision runs first, with its mfa obligation: a remote forward
// needs the second factor the connection passed.
func TestRemoteForwardRequiresMFA(t *testing.T) {
	e := newPipeEnv(t, 10)
	withMFA(e.p)
	e.f.setPolicy(fwdPolicy(t, e.m, "v1", pipePolicies+mfaRuleOn("rfwd", "rfwd", "any")+`
forbid (principal, action == Werbot::Action::"rfwd.remote_forward", resource) when { context.port == 9 };`, true))
	_, err := e.p.Open(t.Context(), e.fwdReq("fwd", "localhost", 9))
	require.ErrorIs(t, err, ErrMFARequired, "connect decides before remote_forward")
	_, err = e.p.Open(t.Context(), e.fwdReq("fwd", "localhost", 8080))
	require.ErrorIs(t, err, ErrMFARequired)

	r := e.fwdReq("fwd", "localhost", 8080)
	r.MFA.Method = "totp"
	s, err := e.p.Open(t.Context(), r)
	require.NoError(t, err)
	e.p.abort(s)
	r = e.fwdReq("fwd", "localhost", 9)
	r.MFA.Method = "totp"
	_, err = e.p.Open(t.Context(), r)
	require.ErrorIs(t, err, ErrDenied)
}

// An agent never gets a remote forward: its baseline permits only exec, and
// its remote_forward forbid holds under a custom permit of everything for
// agents.
func TestRemoteForwardAgentDenied(t *testing.T) {
	for name, policies := range map[string]string{
		"baseline":      agentBaseline(nil, "execy"),
		"custom permit": agentBaseline(nil, "execy") + agentRemoteForwardForbid + `permit (principal is Werbot::Agent, action, resource);`,
	} {
		t.Run(name, func(t *testing.T) {
			e, _ := newAgentEnv(t)
			e.m["rfwd"] = uuid.New()
			s := testSnapshot(t, e.m, pipePolicies+policies, nil, res{"rfwd", "fwd", "rfwd"})
			s.Resources[0].ConfigJson = []byte("true")
			withAgents(t, s, e.m, testAgent{"a1", "u1", "active"})
			p, err := CompilePolicy(s)
			require.NoError(t, err)
			e.f.setPolicy(p)
			_, err = e.p.Open(t.Context(), e.fwdReq("fwd", "localhost", 8080))
			require.ErrorIs(t, err, ErrDenied)
			e.f.with(func(f *fakeControl) { require.Zero(t, f.startCalls) })
		})
	}
}

// agentRemoteForwardForbid is the server's agent:baseline:remote_forward
// policy for the rfwd type.
const agentRemoteForwardForbid = `@id("agent:baseline:remote_forward")
forbid (principal is Werbot::Agent, action in [Werbot::Action::"rfwd.remote_forward"], resource);
`

// The sweeper keeps a remote forward on a new snapshot that still permits it
// and ends it once the flag is off or a forbid covers the action.
func TestRemoteForwardReevaluated(t *testing.T) {
	e := newPipeEnv(t, 10)
	e.f.setPolicy(fwdPolicy(t, e.m, "v1", pipePolicies, true))
	startSweeper(t, e.p)
	run := func() (*Session, <-chan struct{}) {
		s, err := e.p.Open(t.Context(), e.fwdReq("fwd", "localhost", 8080))
		require.NoError(t, err)
		return s, e.runBlocker(t, s)
	}
	ended := func(s *Session, done <-chan struct{}) {
		t.Helper()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("the remote forward was kept")
		}
		requireEnds(t, e.f, s.ID, "access_revoked")
	}

	s, done := run()
	e.f.setPolicy(fwdPolicy(t, e.m, "v2", pipePolicies, true))
	select {
	case <-done:
		t.Fatalf("a permitted remote forward ended: %v", e.f.endsOf(s.ID))
	case <-time.After(200 * time.Millisecond):
	}
	e.f.setPolicy(fwdPolicy(t, e.m, "v3", pipePolicies, false))
	ended(s, done)

	e.f.setPolicy(fwdPolicy(t, e.m, "v4", pipePolicies, true))
	s, done = run()
	e.f.setPolicy(fwdPolicy(t, e.m, "v5", pipePolicies+`
forbid (principal, action == Werbot::Action::"rfwd.remote_forward", resource);`, true))
	ended(s, done)
}
