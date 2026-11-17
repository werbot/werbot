package connector

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	connectorv1 "github.com/werbot/werbot/gen/werbot/connector/v1"
)

func TestValidHost(t *testing.T) {
	assert.True(t, ValidHost("10.0.0.5"))
	assert.True(t, ValidHost(strings.Repeat("a", 253)))
	for _, h := range []string{"", "a b", "a/b", strings.Repeat("a", 254)} {
		assert.False(t, ValidHost(h), h)
	}
}

func TestCheckPorts(t *testing.T) {
	rs, err := CheckPorts([]*connectorv1.PortRange{{Port: 80}, {Port: 8000, EndPort: 8010}})
	require.NoError(t, err)
	assert.Equal(t, []PortRange{{Port: 80}, {Port: 8000, EndPort: 8010}}, rs)

	_, err = CheckPorts(nil)
	assert.Error(t, err)
	_, err = CheckPorts([]*connectorv1.PortRange{{Port: 70000}})
	assert.Error(t, err)
	_, err = CheckPorts([]*connectorv1.PortRange{{Port: 80}, {Port: 80}})
	assert.Error(t, err)
}
