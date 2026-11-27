package gateway

import (
	"net/netip"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// StartSession carries the database of a pre-session, cut to the wire bound.
func TestTunnelStartSessionDBName(t *testing.T) {
	f := &agentTunnel{}
	c := newAgentTunnel(t, f)
	ip := netip.MustParseAddr("192.0.2.7")
	long := strings.Repeat("б", 40) // 80 bytes

	for _, db := range []string{"app", long, ""} {
		_, _, err := c.StartSession(t.Context(), testUserID, uuid.New(), ip, KindStream, "db:5432", "", nil, nil, startArgs{DBName: db})
		require.NoError(t, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	assert.Equal(t, "app", f.starts[0].GetDbName())
	got := f.starts[1].GetDbName()
	assert.Len(t, got, 62, "cut to 63 bytes before a split character")
	assert.True(t, utf8.ValidString(got))
	assert.Empty(t, f.starts[2].GetDbName(), "no pre-session: no database")
}
