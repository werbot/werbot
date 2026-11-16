package gateway

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseLogin(t *testing.T) {
	login, resource, err := ParseLogin("alice")
	require.NoError(t, err)
	require.Equal(t, "alice", login)
	require.Empty(t, resource)

	login, resource, err = ParseLogin("alice+db-1")
	require.NoError(t, err)
	require.Equal(t, "alice", login)
	require.Equal(t, "db-1", resource)

	for _, bad := range []string{"Alice", "a", "alice+", "alice+db+x", "alice+DB", "", "+db"} {
		_, _, err := ParseLogin(bad)
		require.Error(t, err, bad)
	}
}
