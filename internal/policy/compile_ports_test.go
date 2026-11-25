package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/connector"
)

func TestCompileBindingPorts(t *testing.T) {
	s := snap()
	s.Bindings[0].Ports = []connector.PortRange{{Port: 8000, EndPort: 8100}, {Port: 5432}}
	_, text, err := Compile(s)
	require.NoError(t, err)
	assert.Equal(t, `@id("binding:019a0000-0000-7000-8000-000000000004")
permit (principal == Werbot::User::"019a0000-0000-7000-8000-000000000001", action, resource)
when { [Werbot::Resource::"019a0000-0000-7000-8000-000000000003"].contains(resource) }
when { context.port == 0 || (context.port >= 5432 && context.port <= 5432) || (context.port >= 8000 && context.port <= 8100) };

`+blocklistPolicy+"\n"+baselineText(), text)

	_, text, err = Compile(snap())
	require.NoError(t, err)
	assert.NotContains(t, text, "context.port", "a binding without ports allows every port")

	bad := snap()
	bad.Bindings[0].Ports = []connector.PortRange{{Port: 9000, EndPort: 8000}}
	ps, _, err := Compile(bad)
	assert.Error(t, err)
	assert.Nil(t, ps)
}

func TestAuthorizePort(t *testing.T) {
	s := snap()
	s.Bindings[0].Ports = []connector.PortRange{{Port: 5432}}
	for port, allow := range map[uint16]bool{5432: true, 5433: false, 5431: false, 0: true, 65535: false} {
		r := req("10.1.2.3", "DE", "")
		r.Port = port
		d := authorize(t, s, r)
		assert.Equal(t, allow, d.Allow, port)
		if !allow {
			assert.Empty(t, d.Reasons, port)
		}
	}

	r := req("10.1.2.3", "DE", "")
	r.Port = 5433
	assert.True(t, authorize(t, snap(), r).Allow, "a binding without ports allows every port")

	ranged := snap()
	ranged.Bindings[0].Ports = []connector.PortRange{{Port: 8000, EndPort: 8100}}
	for port, allow := range map[uint16]bool{7999: false, 8000: true, 8050: true, 8100: true, 8101: false} {
		r.Port = port
		assert.Equal(t, allow, authorize(t, ranged, r).Allow, port)
	}
}
