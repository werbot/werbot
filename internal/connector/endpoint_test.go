package connector

import (
	"testing"

	"github.com/stretchr/testify/assert"

	connectorv1 "github.com/werbot/werbot/gen/werbot/connector/v1"
)

func TestValidatePorts(t *testing.T) {
	assert.NoError(t, ValidatePorts([]PortRange{{Port: 22}}))
	assert.NoError(t, ValidatePorts([]PortRange{{Port: 8000, EndPort: 8100}, {Port: 9000}}))
	many := make([]PortRange, 17)
	for i := range many {
		many[i] = PortRange{Port: uint16(i + 1)}
	}
	for name, rs := range map[string][]PortRange{
		"empty":    nil,
		"17":       many,
		"zero":     {{Port: 0}},
		"reversed": {{Port: 100, EndPort: 99}},
		"overlap":  {{Port: 1, EndPort: 10}, {Port: 5, EndPort: 20}},
		"dup":      {{Port: 5}, {Port: 5}},
	} {
		assert.Error(t, ValidatePorts(rs), name)
	}
}

func TestPortsFromProto(t *testing.T) {
	got := PortsFromProto([]*connectorv1.PortRange{{Port: 80}, {Port: 8000, EndPort: 8100}})
	assert.Equal(t, []PortRange{{Port: 80}, {Port: 8000, EndPort: 8100}}, got)
	assert.Error(t, ValidatePorts(PortsFromProto([]*connectorv1.PortRange{{Port: 70000}})))
}

func TestEndpointAllow(t *testing.T) {
	e := Endpoint{Host: "10.0.0.5", Ports: []PortRange{{Port: 5432}, {Port: 6000, EndPort: 6010}}}
	assert.True(t, e.Allow("pg", "10.0.0.5", 5432))
	assert.True(t, e.Allow("pg", "PG", 6005))
	assert.False(t, e.Allow("pg", "10.0.0.6", 5432))
	assert.False(t, e.Allow("pg", "pg", 5433))
	assert.True(t, Endpoint{AnyTarget: true}.Allow("x", "any", 1))
}
