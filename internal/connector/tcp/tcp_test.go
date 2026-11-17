package tcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/wrapperspb"

	tcpv1 "github.com/werbot/werbot/gen/werbot/connector/tcp/v1"
	connectorv1 "github.com/werbot/werbot/gen/werbot/connector/v1"
	"github.com/werbot/werbot/internal/connector"
)

func TestTCPRegistered(t *testing.T) {
	ty, ok := connector.Lookup(TypeName)
	require.True(t, ok)
	assert.Equal(t, "tcp", ty.Name)
	assert.Equal(t, connector.CategoryServers, ty.Category)
	assert.Equal(t, []string{"tcp.connect"}, ty.Actions)
	assert.Empty(t, ty.Credentials)
	assert.Equal(t, "L1", ty.Level)
	assert.Equal(t, []connector.Kind{connector.KindStream}, ty.Kinds)
	assert.Empty(t, ty.Recorded)

	e, ok, err := ty.EndpointOf([]byte(`{"host":"db","ports":[{"port":5432},{"port":8000,"endPort":8010}]}`))
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, connector.Endpoint{Host: "db", Ports: []connector.PortRange{{Port: 5432}, {Port: 8000, EndPort: 8010}}}, e)
	assert.True(t, e.Allow("r", "db", 8005))
	assert.False(t, e.Allow("r", "db", 22))
}

func TestTCPValidate(t *testing.T) {
	ok := []*connectorv1.PortRange{{Port: 80}}
	require.NoError(t, Validate(&tcpv1.Config{Host: "h", Ports: ok}))

	assert.Error(t, Validate(&tcpv1.Config{Ports: ok}))
	assert.Error(t, Validate(&tcpv1.Config{Host: "a b", Ports: ok}))
	assert.Error(t, Validate(&tcpv1.Config{Host: "h"}))
	assert.Error(t, Validate(&tcpv1.Config{Host: "h", Ports: []*connectorv1.PortRange{{Port: 0}}}))
	assert.Error(t, Validate(&tcpv1.Config{Host: "h", Ports: []*connectorv1.PortRange{{Port: 70000}}}))
	assert.Error(t, Validate(&tcpv1.Config{Host: "h", Ports: []*connectorv1.PortRange{{Port: 10, EndPort: 5}}}))
	assert.Error(t, Validate(&tcpv1.Config{Host: "h", Ports: []*connectorv1.PortRange{{Port: 10, EndPort: 20}, {Port: 15}}}))
	assert.Error(t, Validate(&tcpv1.Config{Host: "[::1]", Ports: ok}))
	assert.Error(t, Validate(&tcpv1.Config{Host: "h:80", Ports: ok}))
	require.NoError(t, Validate(&tcpv1.Config{Host: "::1", Ports: ok}))
	assert.Error(t, Validate(wrapperspb.String("x")))
}

func TestTCPEndpointFailsClosedOnBadPorts(t *testing.T) {
	ty, _ := connector.Lookup(TypeName)
	e := ty.Endpoint(&tcpv1.Config{Host: "h", Ports: []*connectorv1.PortRange{{Port: 70000}}})
	assert.False(t, e.Allow("r", "h", 80))
}

func TestRangeLimit(t *testing.T) {
	rs := func(n int) []*connectorv1.PortRange {
		out := make([]*connectorv1.PortRange, n)
		for i := range out {
			out[i] = &connectorv1.PortRange{Port: uint32(i + 1)}
		}
		return out
	}
	require.NoError(t, Validate(&tcpv1.Config{Host: "h", Ports: rs(16)}))
	assert.Error(t, Validate(&tcpv1.Config{Host: "h", Ports: rs(17)}))
}
