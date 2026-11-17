package recorder_test

import (
	"bytes"
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

func TestSpoolRecoversAfterRestart(t *testing.T) {
	for _, ended := range []bool{false, true} {
		t.Run(fmt.Sprintf("ended=%v", ended), func(t *testing.T) {
			dir := t.TempDir()
			down := &fakeUploader{}
			down.down.Store(true)
			old := openSpool(t, dir, down, 0)
			id := uuid.New()
			total, last := appendN(t, old, id, 2)
			if ended {
				require.NoError(t, old.End(id, "gateway_shutdown"))
			}

			up := &fakeUploader{}
			sp := openSpool(t, dir, up, 0)
			require.Equal(t, total, sp.Total())
			require.Equal(t, total, sp.Pending(id))
			want := []string{fmt.Sprintf("chunk %s 0", id), fmt.Sprintf("chunk %s 1", id), fmt.Sprintf("finish %s 2", id)}
			if ended {
				require.Equal(t, []uuid.UUID{id}, sp.PendingEnds())
				want = append([]string{fmt.Sprintf("end %s gateway_shutdown", id)}, want...)
			} else {
				require.Empty(t, sp.PendingEnds())
			}
			runSpool(t, sp)
			require.Eventually(t, func() bool { return slices.Equal(up.journal(), want) }, 5*time.Second, 5*time.Millisecond)
			require.Eventually(t, gone(filepath.Join(dir, id.String())), 5*time.Second, 5*time.Millisecond)
			require.Equal(t, []recorder.Manifest{{SessionID: id, Chunks: 2, Bytes: total, FinalHash: last}}, up.finishes)
			require.Zero(t, sp.Total())
		})
	}
}

// A crash between the chunk file and state.json leaves a chunk the state does not
// count yet; recovery counts it.
func TestSpoolRecoversChunkWrittenBeforeState(t *testing.T) {
	dir := t.TempDir()
	old := openSpool(t, dir, &fakeUploader{}, 0)
	id := uuid.New()
	_, _ = appendN(t, old, id, 1)
	sdir, err := os.OpenRoot(filepath.Join(dir, id.String()))
	require.NoError(t, err)
	state, err := sdir.ReadFile("state.json")
	require.NoError(t, err)
	hash := bytes.Repeat([]byte{9}, 32)
	require.NoError(t, old.Append(id, 1, hash, make([]byte, 40)))
	require.NoError(t, sdir.WriteFile("state.json", state, 0o600))
	require.NoError(t, sdir.WriteFile("00000002.chunk.tmp", []byte("torn"), 0o600))

	up := &fakeUploader{}
	sp := openSpool(t, dir, up, 0)
	require.EqualValues(t, 100+40, sp.Total())
	runSpool(t, sp)
	require.Eventually(t, func() bool { return up.count(fmt.Sprintf("finish %s 2", id)) == 1 }, 5*time.Second, 5*time.Millisecond)
	require.Equal(t, hash, up.finishes[0].FinalHash)
	require.EqualValues(t, 140, up.finishes[0].Bytes)
}

// A chunk whose removal after upload failed is not mistaken for an unsent one:
// uploads run in order, so a missing chunk means every earlier one was sent too.
func TestSpoolRestoreSkipsLeftoverUploadedChunk(t *testing.T) {
	dir := t.TempDir()
	old := openSpool(t, dir, &fakeUploader{}, 0)
	id := uuid.New()
	_, last := appendN(t, old, id, 4) // gz sizes 100..103
	require.NoError(t, old.Close(id))
	sdir, err := os.OpenRoot(filepath.Join(dir, id.String()))
	require.NoError(t, err)
	require.NoError(t, sdir.Remove("00000001.chunk")) // 0 left behind, 1 removed, 2 and 3 unsent

	up := &fakeUploader{}
	sp := openSpool(t, dir, up, 0)
	require.EqualValues(t, 102+103, sp.Total())
	runSpool(t, sp)
	want := []string{fmt.Sprintf("chunk %s 2", id), fmt.Sprintf("chunk %s 3", id), fmt.Sprintf("finish %s 4", id)}
	require.Eventually(t, func() bool { return slices.Equal(up.journal(), want) }, 5*time.Second, 5*time.Millisecond)
	require.Equal(t, last, up.finishes[0].FinalHash)
	require.Eventually(t, gone(filepath.Join(dir, id.String())), 5*time.Second, 5*time.Millisecond)
	require.NoDirExists(t, filepath.Join(dir, "rejected"))
}
