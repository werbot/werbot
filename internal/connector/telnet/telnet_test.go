package telnet

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/wrapperspb"

	telnetv1 "github.com/werbot/werbot/gen/werbot/connector/telnet/v1"
	"github.com/werbot/werbot/internal/connector"
)

func TestTelnetRegistered(t *testing.T) {
	ty, ok := connector.Lookup(TypeName)
	require.True(t, ok)
	assert.Equal(t, "telnet", ty.Name)
	assert.Equal(t, connector.CategoryServers, ty.Category)
	assert.Equal(t, []string{"telnet.connect"}, ty.Actions)
	assert.Equal(t, []connector.CredentialKind{connector.CredentialPassword}, ty.Credentials)
	assert.Equal(t, "L2", ty.Level)
	assert.Equal(t, []connector.Kind{connector.KindTerminal, connector.KindStream}, ty.Kinds)
	assert.Equal(t, []connector.Kind{connector.KindTerminal, connector.KindStream}, ty.Recorded)

	e, ok, err := ty.EndpointOf([]byte(`{"host":"sw1"}`))
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, connector.Endpoint{Host: "sw1", Ports: []connector.PortRange{{Port: 23}}}, e)
	e, _, err = ty.EndpointOf([]byte(`{"host":"sw1","port":2323}`))
	require.NoError(t, err)
	assert.Equal(t, []connector.PortRange{{Port: 2323}}, e.Ports)
}

func TestTelnetValidate(t *testing.T) {
	c := &telnetv1.Config{Host: "10.0.0.5"}
	require.NoError(t, Validate(c))
	assert.EqualValues(t, 23, c.Port)
	assert.Empty(t, c.Login)

	require.NoError(t, Validate(&telnetv1.Config{Host: "h", Port: 65535, Login: "admin"}))
	assert.Error(t, Validate(&telnetv1.Config{}))
	assert.Error(t, Validate(&telnetv1.Config{Host: "a b"}))
	assert.Error(t, Validate(&telnetv1.Config{Host: "a/b"}))
	assert.Error(t, Validate(&telnetv1.Config{Host: "h", Port: 65536}))
	assert.Error(t, Validate(&telnetv1.Config{Host: "h", Login: "a b"}))
	assert.Error(t, Validate(&telnetv1.Config{Host: "h", Login: strings.Repeat("a", 65)}))
	assert.Error(t, Validate(&telnetv1.Config{Host: "h", Login: "admin\r\n"}))
	assert.Error(t, Validate(&telnetv1.Config{Host: "h\n"}))
	assert.Error(t, Validate(&telnetv1.Config{Host: "[::1]"}))
	require.NoError(t, Validate(&telnetv1.Config{Host: "fe80::1"}))
	assert.Error(t, Validate(wrapperspb.String("x")))
}
