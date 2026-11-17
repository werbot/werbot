package udp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	udpv1 "github.com/werbot/werbot/gen/werbot/connector/udp/v1"
	connectorv1 "github.com/werbot/werbot/gen/werbot/connector/v1"
	"github.com/werbot/werbot/internal/connector"
)

var ports = []*connectorv1.PortRange{{Port: 53}}

func TestUDPRegistered(t *testing.T) {
	ty, ok := connector.Lookup(TypeName)
	require.True(t, ok)
	assert.Equal(t, "udp", ty.Name)
	assert.Equal(t, connector.CategoryServers, ty.Category)
	assert.Equal(t, []string{"udp.connect"}, ty.Actions)
	assert.Empty(t, ty.Credentials)
	assert.Equal(t, "L1", ty.Level)
	assert.Equal(t, []connector.Kind{connector.KindDatagram}, ty.Kinds)
	assert.Empty(t, ty.Recorded)

	e, ok, err := ty.EndpointOf([]byte(`{"host":"dns","ports":[{"port":53}]}`))
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, connector.Endpoint{Host: "dns", Ports: []connector.PortRange{{Port: 53}}, NoHealth: true}, e)
}

func TestUDPValidate(t *testing.T) {
	c := &udpv1.Config{Host: "h", Ports: ports}
	require.NoError(t, Validate(c))
	assert.Equal(t, 2*time.Minute, c.FlowIdle.AsDuration())

	c = &udpv1.Config{Host: "h", Ports: ports, FlowIdle: durationpb.New(0)}
	require.NoError(t, Validate(c))
	assert.Equal(t, 2*time.Minute, c.FlowIdle.AsDuration())

	for _, d := range []time.Duration{time.Second, 30 * time.Second, time.Hour} {
		c = &udpv1.Config{Host: "h", Ports: ports, FlowIdle: durationpb.New(d)}
		require.NoError(t, Validate(c), d)
		assert.Equal(t, d, c.FlowIdle.AsDuration())
	}
	for _, d := range []time.Duration{500 * time.Millisecond, 2 * time.Hour, -time.Second} {
		assert.Error(t, Validate(&udpv1.Config{Host: "h", Ports: ports, FlowIdle: durationpb.New(d)}), d)
	}
	assert.Error(t, Validate(&udpv1.Config{Host: "h", Ports: ports, FlowIdle: &durationpb.Duration{Seconds: 1, Nanos: -5}}))

	assert.Error(t, Validate(&udpv1.Config{Ports: ports}))
	assert.Error(t, Validate(&udpv1.Config{Host: "h"}))
	assert.Error(t, Validate(&udpv1.Config{Host: "h", Ports: []*connectorv1.PortRange{{Port: 70000}}}))
	assert.Error(t, Validate(&udpv1.Config{Host: "[::1]", Ports: ports}))
	assert.Error(t, Validate(&udpv1.Config{Host: "h:80", Ports: ports}))
	require.NoError(t, Validate(&udpv1.Config{Host: "::1", Ports: ports}))
	assert.Error(t, Validate(wrapperspb.String("x")))
}

func TestRangeLimit(t *testing.T) {
	rs := func(n int) []*connectorv1.PortRange {
		out := make([]*connectorv1.PortRange, n)
		for i := range out {
			out[i] = &connectorv1.PortRange{Port: uint32(i + 1)}
		}
		return out
	}
	require.NoError(t, Validate(&udpv1.Config{Host: "h", Ports: rs(16)}))
	assert.Error(t, Validate(&udpv1.Config{Host: "h", Ports: rs(17)}))
}
