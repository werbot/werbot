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

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
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
				require.NoError(t, old.End(id, "gateway_shutdown", nil))
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

// A state write that fails keeps the chunk already on disk: restore counts it, so
// a restart never mistakes the earlier chunks for sent ones.
func TestSpoolFailedStateKeepsChunk(t *testing.T) {
	dir := t.TempDir()
	old := openSpool(t, dir, &fakeUploader{}, 0)
	id := uuid.New()
	_, _ = appendN(t, old, id, 1)
	require.NoError(t, os.Mkdir(filepath.Join(dir, id.String(), "state.json.tmp"), 0o700)) // the next state write fails
	hash := bytes.Repeat([]byte{7}, 32)
	require.Error(t, old.Append(id, 1, hash, make([]byte, 40)))
	require.FileExists(t, filepath.Join(dir, id.String(), "00000001.chunk"))
	require.NoError(t, os.Remove(filepath.Join(dir, id.String(), "state.json.tmp")))

	up := &fakeUploader{}
	sp := openSpool(t, dir, up, 0)
	require.EqualValues(t, 100+40, sp.Total())
	runSpool(t, sp)
	require.Eventually(t, func() bool { return up.count(fmt.Sprintf("finish %s 2", id)) == 1 }, 5*time.Second, 5*time.Millisecond)
	require.Equal(t, hash, up.finishes[0].FinalHash)
}

// A chunk file above a gap cannot join the chain (its predecessor is gone): restore
// drops it instead of counting it as unsent forever.
func TestSpoolRestoreDropsChunkAboveGap(t *testing.T) {
	dir := t.TempDir()
	old := openSpool(t, dir, &fakeUploader{}, 0)
	id := uuid.New()
	_, last := appendN(t, old, id, 2)
	stray := filepath.Join(dir, id.String(), "00000005.chunk")
	require.NoError(t, os.WriteFile(stray, make([]byte, 64), 0o600))

	up := &fakeUploader{}
	sp := openSpool(t, dir, up, 0)
	require.EqualValues(t, 100+101, sp.Total())
	require.NoFileExists(t, stray)
	runSpool(t, sp)
	require.Eventually(t, func() bool { return up.count(fmt.Sprintf("finish %s 2", id)) == 1 }, 5*time.Second, 5*time.Millisecond)
	require.Equal(t, last, up.finishes[0].FinalHash)
	require.Zero(t, sp.Total())
}

// end.json keeps the final counters across a restart; one written before counters
// existed restores without them.
func TestSpoolEndCountersSurviveRestart(t *testing.T) {
	dir := t.TempDir()
	down := &fakeUploader{}
	down.down.Store(true)
	old := openSpool(t, dir, down, 0)
	a, b := uuid.New(), uuid.New()
	require.NoError(t, old.End(a, "idle_timeout", &gatewayv1.SessionCounters{SessionId: a.String(), BytesUp: 5, BytesDown: 6, DatagramsUp: 1, DatagramsDown: 2}))
	require.NoError(t, os.Mkdir(filepath.Join(dir, b.String()), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, b.String(), "end.json"), []byte(`{"reason":"closed"}`), 0o600))

	up := &fakeUploader{}
	sp := openSpool(t, dir, up, 0)
	runSpool(t, sp)
	want := []string{fmt.Sprintf("end %s idle_timeout %s 5/6/1/2", a, a), fmt.Sprintf("end %s closed", b)}
	require.Eventually(t, func() bool {
		return slices.Equal(slices.Sorted(slices.Values(up.journal())), slices.Sorted(slices.Values(want)))
	},
		5*time.Second, 5*time.Millisecond)
}
