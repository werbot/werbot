package connectortest

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/gateway"
)

func init() {
	connector.Register(connector.Type{
		Name: "ctfake", NewConfig: func() proto.Message { return &emptypb.Empty{} },
		Validate: func(proto.Message) error { return nil }, Actions: []string{"ctfake.connect"},
		Kinds:    []connector.Kind{connector.KindTerminal, connector.KindStream},
		Recorded: []connector.Kind{connector.KindTerminal},
		Endpoint: func(proto.Message) connector.Endpoint {
			return connector.Endpoint{Host: "db.example", Ports: []connector.PortRange{{Port: 5432}}}
		},
	})
}

// failT records a failed require instead of ending the test.
type failT struct {
	msg    string
	failed bool
}

func (f *failT) Errorf(format string, args ...any) { f.msg = fmt.Sprintf(format, args...) }
func (f *failT) FailNow()                          { f.failed = true; panic(f) }

// check runs fn and reports its failure; any other panic goes on.
func check(fn func(t *failT)) (f *failT) {
	f = &failT{}
	defer func() {
		if r := recover(); r != nil && r != any(f) {
			panic(r)
		}
	}()
	fn(f)
	return f
}

func spy(output, input bool) *spySink {
	s := &spySink{}
	s.output.Store(output)
	s.input.Store(input)
	return s
}

// A connector that records a kind its type does not record fails; a recorded kind needs traffic.
func TestRunRequiresRecordingOnlyForRecorded(t *testing.T) {
	f := check(func(f *failT) { requireRecorded(f, "ctfake", gateway.KindStream, spy(true, false)) })
	require.True(t, f.failed)
	require.Contains(t, f.msg, "which its type does not record")
	require.False(t, check(func(f *failT) { requireRecorded(f, "ctfake", gateway.KindStream, spy(false, false)) }).failed)

	f = check(func(f *failT) { requireRecorded(f, "ctfake", gateway.KindTerminal, spy(false, true)) })
	require.True(t, f.failed)
	require.Contains(t, f.msg, "Recording.Output was not called")
	f = check(func(f *failT) { requireRecorded(f, "ctfake", gateway.KindTerminal, spy(true, false)) })
	require.True(t, f.failed)
	require.Contains(t, f.msg, "Recording.Input was not called")
	require.False(t, check(func(f *failT) { requireRecorded(f, "ctfake", gateway.KindTerminal, spy(true, true)) }).failed)
}

func TestRequireEndpointAllows(t *testing.T) {
	res := gateway.Resource{Name: "db1", Type: "ctfake", Config: []byte("{}")}
	require.False(t, check(func(f *failT) { requireEndpointAllows(f, "ctfake", res, "db.example:5432") }).failed)
	require.False(t, check(func(f *failT) { requireEndpointAllows(f, "ctfake", res, "db1:5432") }).failed)
	f := check(func(f *failT) { requireEndpointAllows(f, "ctfake", res, "db.example:22") })
	require.True(t, f.failed)
	require.Contains(t, f.msg, "does not allow")
	require.False(t, check(func(f *failT) { requireEndpointAllows(f, "unregistered", res, "x:1") }).failed)
}

type marked struct{ Echo }

func (marked) DialsTarget() {}

func TestRunDialsThroughSession(t *testing.T) {
	require.Contains(t, check(func(f *failT) { requireDialsThroughSession(f, marked{}, 0) }).msg, "must dial")
	require.Contains(t, check(func(f *failT) { requireDialsThroughSession(f, Echo{}, 1) }).msg, "lacks DialsTarget")
	require.False(t, check(func(f *failT) { requireDialsThroughSession(f, marked{}, 1) }).failed)
	require.False(t, check(func(f *failT) { requireDialsThroughSession(f, Echo{}, 0) }).failed)
}

// One Session.Dial plus one direct connection is a dial-around; only Session.Dial passes.
func TestRequireTargetConnections(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	l, accepts := CountAccepts(l)
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { _ = c.Close() })
		}
	}()
	dial := func() {
		c, err := net.Dial("tcp", l.Addr().String())
		require.NoError(t, err)
		t.Cleanup(func() { _ = c.Close() })
	}
	dial() // the one through Session.Dial
	require.False(t, check(func(f *failT) { requireTargetConnections(f, accepts, 0, 1) }).failed)
	dial() // a direct one
	f := check(func(f *failT) { requireTargetConnections(f, accepts, 0, 1) })
	require.True(t, f.failed)
	require.Contains(t, f.msg, "did not come through Session.Dial")
}

type udpOnly struct{ Echo }

func (udpOnly) Kinds() []gateway.Kind { return []gateway.Kind{gateway.KindDatagram} }
func (udpOnly) DialsTarget()          {}

func TestRequireAcceptsFit(t *testing.T) {
	tcp := []gateway.Kind{gateway.KindStream}
	udp := []gateway.Kind{gateway.KindDatagram}
	counter := func() int { return 0 }
	f := check(func(f *failT) { requireAcceptsFit(f, marked{}, tcp, nil) })
	require.True(t, f.failed)
	require.Contains(t, f.msg, "needs Target.Accepts")
	require.False(t, check(func(f *failT) { requireAcceptsFit(f, marked{}, tcp, counter) }).failed)
	require.False(t, check(func(f *failT) { requireAcceptsFit(f, Echo{}, tcp, nil) }).failed) // not a TargetDialer
	require.False(t, check(func(f *failT) { requireAcceptsFit(f, udpOnly{}, udp, nil) }).failed)
	f = check(func(f *failT) { requireAcceptsFit(f, udpOnly{}, udp, counter) })
	require.True(t, f.failed)
	require.Contains(t, f.msg, "UDP-only")
}
