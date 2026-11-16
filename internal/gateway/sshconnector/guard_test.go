package sshconnector

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway"
)

// A detached relay that panics after its session ended is logged, not lost,
// with the session secret redacted.
func TestGuardLogsPanicAfterDone(t *testing.T) {
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	g := &guard{stop: func() {}, secret: gateway.Secret{Kind: "password", Value: []byte("s3cret-value")}}
	require.NoError(t, g.done())
	g.call(func() error { panic("late fault s3cret-value") })
	require.Contains(t, logs.String(), "relay panic after the session ended")
	require.Contains(t, logs.String(), "late fault [redacted]")
	require.NotContains(t, logs.String(), "s3cret-value")
}
