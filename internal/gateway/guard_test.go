package gateway

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

// A detached relay that panics after its session ended is logged, not lost,
// with the session secret redacted.
func TestGuardLogsPanicAfterWait(t *testing.T) {
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	g := NewGuard(func() {}, Secret{Kind: "password", Value: []byte("s3cret-value")})
	require.NoError(t, g.Wait())
	g.call(func() error { panic("late fault s3cret-value") })
	require.Contains(t, logs.String(), "relay panic after the session ended")
	require.Contains(t, logs.String(), "late fault [redacted]")
	require.NotContains(t, logs.String(), "s3cret-value")
}
