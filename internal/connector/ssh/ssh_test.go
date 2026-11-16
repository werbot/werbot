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
	assert.Equal(t, []string{"ssh.connect"}, ty.Actions)
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
	assert.Error(t, Validate(wrapperspb.String("x")))
}
