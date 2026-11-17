package connectcli

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The table holds 32 flows; a new address beyond them is dropped until a flow
// is idle for flowIdle. A known address keeps its id.
func TestFlowTable(t *testing.T) {
	r := newUDPRelay(&binding{})
	now := time.Now()
	addr := func(port int) net.Addr { return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port} }
	for i := range maxFlows {
		id, ok := r.flowFor(addr(1000+i), now)
		require.True(t, ok)
		assert.Equal(t, uint32(i+1), id)
	}
	_, ok := r.flowFor(addr(2000), now.Add(flowIdle-time.Second))
	assert.False(t, ok, "the 33rd flow is dropped")
	id, ok := r.flowFor(addr(1000), now.Add(time.Minute))
	assert.True(t, ok)
	assert.Equal(t, uint32(1), id, "a known address keeps its flow")

	id, ok = r.flowFor(addr(2000), now.Add(flowIdle))
	assert.True(t, ok, "flows idle for flowIdle are freed")
	assert.Equal(t, uint32(maxFlows+1), id)
	assert.Len(t, r.flows, 2, "flow 1 was seen a minute later and stays")
	assert.Equal(t, r.flows[addr(2000).String()], r.byID[id])
}
