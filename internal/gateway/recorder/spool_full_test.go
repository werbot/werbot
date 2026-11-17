//go:build unix

package recorder_test

import (
	"syscall"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// A disk with room for an empty file but not for a chunk keeps the spool
// unwritable. RLIMIT_FSIZE stands in for a full disk: a write past the limit
// fails with EFBIG (the Go runtime ignores SIGXFSZ).
func TestSpoolWritableNeedsRoomForAChunk(t *testing.T) {
	sp := openSpool(t, t.TempDir(), &fakeUploader{}, 0)
	var lim syscall.Rlimit
	require.NoError(t, syscall.Getrlimit(syscall.RLIMIT_FSIZE, &lim))
	full := lim
	full.Cur = 1 << 10
	require.NoError(t, syscall.Setrlimit(syscall.RLIMIT_FSIZE, &full))
	restore := func() { _ = syscall.Setrlimit(syscall.RLIMIT_FSIZE, &lim) }
	t.Cleanup(restore)

	require.Error(t, sp.Append(uuid.New(), 0, make([]byte, 32), make([]byte, 4<<10)), "the chunk does not fit")
	require.False(t, sp.Writable(), "an empty probe file still fits")
	restore()
	require.True(t, sp.Writable())
}
