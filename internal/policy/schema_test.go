package policy_test

import (
	"testing"

	"github.com/cedar-policy/cedar-go"
	"github.com/cedar-policy/cedar-go/x/exp/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/werbot/werbot/internal/connector/ssh"
	"github.com/werbot/werbot/internal/policy"
)

func TestSchemaHasConnectorActions(t *testing.T) {
	var s schema.Schema
	require.NoError(t, s.UnmarshalCedar([]byte(policy.Schema())))
	res, err := s.Resolve()
	require.NoError(t, err)
	assert.Contains(t, res.Actions, cedar.NewEntityUID("Werbot::Action", "ssh.connect"))
}
