package ssh

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/wrapperspb"

	sshv1 "github.com/werbot/werbot/gen/werbot/connector/ssh/v1"
	"github.com/werbot/werbot/internal/connector"
)

func TestSSHRegistered(t *testing.T) {
	ty, ok := connector.Lookup("ssh")
	require.True(t, ok)
	assert.Equal(t, connector.CategoryServers, ty.Category)
	assert.Equal(t, []string{"ssh.connect", "ssh.exec", "ssh.shell", "ssh.remote_forward"}, ty.Actions, "connect first: CheckAccess uses Actions[0]")
	a, ok := ty.ExecAction()
	assert.True(t, ok)
	assert.Equal(t, "ssh.exec", a)
	a, ok = ty.ShellAction()
	assert.True(t, ok)
	assert.Equal(t, "ssh.shell", a)
	assert.Equal(t, []connector.CredentialKind{connector.CredentialPassword, connector.CredentialPrivateKey}, ty.Credentials)
	assert.Equal(t, "L2", ty.Level)
	assert.Equal(t, []connector.Kind{connector.KindTerminal, connector.KindExec, connector.KindStream}, ty.Kinds)
}

func TestSSHValidate(t *testing.T) {
	c := &sshv1.Config{Host: "10.0.0.5", Login: "root"}
	require.NoError(t, Validate(c))
	assert.EqualValues(t, 22, c.Port)

	assert.Error(t, Validate(&sshv1.Config{Login: "root"}))
	assert.Error(t, Validate(&sshv1.Config{Host: "h", Port: 70000, Login: "root"}))
	assert.Error(t, Validate(&sshv1.Config{Host: "h"}))
	assert.Error(t, Validate(&sshv1.Config{Host: "a b", Login: "root"}))
	assert.Error(t, Validate(&sshv1.Config{Host: "h", Login: "root\r\n"}))
	assert.Error(t, Validate(wrapperspb.String("x")))
}

func TestSSHEndpointAndRecorded(t *testing.T) {
	ty, ok := connector.Lookup("ssh")
	require.True(t, ok)
	assert.Equal(t, []connector.Kind{connector.KindTerminal, connector.KindExec}, ty.Recorded)
	assert.False(t, ty.Records(connector.KindStream))

	e, ok, err := ty.EndpointOf([]byte(`{"host":"h","login":"u"}`))
	require.NoError(t, err)
	require.True(t, ok)
	assert.False(t, e.AnyTarget)
	assert.True(t, e.NoTarget, "no port forwarding: no stream target")
	assert.False(t, e.Allow("r", "h", 22))
	assert.Equal(t, "h", e.Host)
	assert.Equal(t, []connector.PortRange{{Port: 22}}, e.Ports, "the port stays for health")

	e, _, err = ty.EndpointOf([]byte(`{"host":"h","login":"u","port":2200,"portForwarding":true}`))
	require.NoError(t, err)
	assert.True(t, e.AnyTarget)
	assert.False(t, e.NoTarget)
	assert.Equal(t, []connector.PortRange{{Port: 2200}}, e.Ports)
}

func TestSSHRemoteForwarding(t *testing.T) {
	ty, _ := connector.Lookup("ssh")
	e, _, err := ty.EndpointOf([]byte(`{"host":"h","login":"u"}`))
	require.NoError(t, err)
	assert.False(t, e.RemoteForward, "off by default")

	e, _, err = ty.EndpointOf([]byte(`{"host":"h","login":"u","remoteForwarding":true}`))
	require.NoError(t, err)
	assert.True(t, e.RemoteForward)
	assert.True(t, e.NoTarget, "a remote forward opens no stream target")
	assert.Contains(t, ty.Actions, "ssh.remote_forward")
}

func TestSSHEndpointDefaultsPortWithoutValidate(t *testing.T) {
	ty, _ := connector.Lookup("ssh")
	e := ty.Endpoint(&sshv1.Config{Host: "h"})
	assert.Equal(t, []connector.PortRange{{Port: 22}}, e.Ports)
}
