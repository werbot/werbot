package gateway

import (
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway/recorder"
)

// EndSession sends a console result within the wire bounds: the command cut
// at a UTF-8 boundary to 65536 bytes, at most 256 paths; no result sends none.
func TestTunnelEndSessionResult(t *testing.T) {
	e := newTunnelEnv(t, nil, time.Hour)
	long := strings.Repeat("я", 40000) // 80000 bytes
	paths := make([]string, 300)
	for i := range paths {
		paths[i] = "/srv/" + strings.Repeat("p", i%10)
	}
	require.NoError(t, e.c.EndSession(t.Context(), testSessionID, "closed", nil,
		&recorder.SessionResult{Command: long, Classes: []string{"read"}, Paths: paths}))
	require.NoError(t, e.c.EndSession(t.Context(), testSessionID, "closed", nil, nil))
	e.f.with(func(f *fakeTunnel) {
		got := f.ends[0]
		require.LessOrEqual(t, len(got.GetCommand()), 65536)
		require.True(t, utf8.ValidString(got.GetCommand()))
		require.True(t, strings.HasPrefix(long, got.GetCommand()))
		require.Equal(t, []string{"read"}, got.GetClasses())
		require.True(t, slices.Equal(paths[:256], got.GetPaths()))
		require.Empty(t, f.ends[1].GetCommand())
		require.Nil(t, f.ends[1].GetClasses())
		require.Nil(t, f.ends[1].GetPaths())
	})
}

// EndSession sends the querylog line count of the result; none sends zero.
func TestTunnelEndSessionQueriesSent(t *testing.T) {
	e := newTunnelEnv(t, nil, time.Hour)
	require.NoError(t, e.c.EndSession(t.Context(), testSessionID, "closed", nil, &recorder.SessionResult{Queries: 4}))
	require.NoError(t, e.c.EndSession(t.Context(), testSessionID, "closed", nil, nil))
	e.f.with(func(f *fakeTunnel) {
		require.EqualValues(t, 4, f.ends[0].GetQueries())
		require.Zero(t, f.ends[1].GetQueries())
	})
}
