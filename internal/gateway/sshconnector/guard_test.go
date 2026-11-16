package sshconnector

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

// A detached relay that panics after its session ended is logged, not lost.
func TestGuardLogsPanicAfterDone(t *testing.T) {
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	g := &guard{stop: func() {}}
	require.NoError(t, g.done())
	g.call(func() error { panic("late fault") })
	require.Contains(t, logs.String(), "relay panic after the session ended")
	require.Contains(t, logs.String(), "late fault")
}
