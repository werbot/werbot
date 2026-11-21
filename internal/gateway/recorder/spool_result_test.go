package recorder_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway/recorder"
)

// end.json keeps a console session's result across a restart; one without a
// result restores none.
func TestSpoolEndResultSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	down := &fakeUploader{}
	down.down.Store(true)
	old := openSpool(t, dir, down, 0)
	a, b := uuid.New(), uuid.New()
	require.NoError(t, old.End(a, "closed", nil, &recorder.SessionResult{Command: "cp a b", Classes: []string{"read", "write"}, Paths: []string{"/srv/a", "/srv/b"}}))
	require.NoError(t, os.Mkdir(filepath.Join(dir, b.String()), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, b.String(), "end.json"), []byte(`{"reason":"closed"}`), 0o600))

	up := &fakeUploader{}
	runSpool(t, openSpool(t, dir, up, 0))
	want := []string{fmt.Sprintf(`end %s closed "cp a b" [read write] [/srv/a /srv/b]`, a), fmt.Sprintf("end %s closed", b)}
	require.Eventually(t, func() bool {
		return slices.Equal(slices.Sorted(slices.Values(up.journal())), slices.Sorted(slices.Values(want)))
	}, 5*time.Second, 5*time.Millisecond)
}
