package recorder_test

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway/recorder"
)

func TestSpoolRetriesUntilServerBack(t *testing.T) {
	dir := t.TempDir()
	up := &fakeUploader{}
	up.down.Store(true)
	sp := openSpool(t, dir, up, 0)
	id := uuid.New()
	total, last := appendN(t, sp, id, 3)
	require.NoError(t, sp.End(id, "closed", nil))
	require.NoError(t, sp.Close(id))
	runSpool(t, sp)

	time.Sleep(50 * time.Millisecond)
	require.FileExists(t, filepath.Join(dir, id.String(), "00000000.chunk"))
	require.Equal(t, total, sp.Pending(id))
	up.down.Store(false)
	want := []string{fmt.Sprintf("end %s closed", id), fmt.Sprintf("chunk %s 0", id), fmt.Sprintf("chunk %s 1", id),
		fmt.Sprintf("chunk %s 2", id), fmt.Sprintf("finish %s 3", id)}
	require.Eventually(t, func() bool { return slices.Equal(up.journal(), want) }, 5*time.Second, 5*time.Millisecond)
	require.Eventually(t, gone(filepath.Join(dir, id.String())), 5*time.Second, 5*time.Millisecond)
	require.Equal(t, []recorder.Manifest{{SessionID: id, Chunks: 3, Bytes: total, FinalHash: last}}, up.finishes)
	require.Zero(t, sp.Total())
	require.Zero(t, sp.Pending(id))
}

func TestSpoolEndSurvivesServerOutage(t *testing.T) {
	up := &fakeUploader{}
	up.down.Store(true)
	sp := openSpool(t, t.TempDir(), up, 0)
	id := uuid.New()
	_, _ = appendN(t, sp, id, 1)
	require.NoError(t, sp.End(id, "closed", nil))
	require.NoError(t, sp.Close(id))
	runSpool(t, sp)
	require.Equal(t, []uuid.UUID{id}, sp.PendingEnds())
	time.Sleep(300 * time.Millisecond)
	require.Equal(t, []uuid.UUID{id}, sp.PendingEnds())
	require.GreaterOrEqual(t, up.endCalls.Load(), int32(2), "EndSession is retried")

	up.down.Store(false)
	end, finish := fmt.Sprintf("end %s closed", id), fmt.Sprintf("finish %s 1", id)
	require.Eventually(t, func() bool { return up.count(finish) == 1 }, 5*time.Second, 5*time.Millisecond)
	j := up.journal()
	require.Equal(t, 1, up.count(end))
	require.Less(t, slices.Index(j, end), slices.Index(j, finish))
	require.Empty(t, sp.PendingEnds())
}

func TestSpoolStoreDownKeepsChunks(t *testing.T) {
	dir := t.TempDir()
	up := &fakeUploader{}
	up.setChunkCode(func(int) connect.Code { return connect.CodeUnavailable }) // the server's store is down
	sp := openSpool(t, dir, up, 0)
	a, b := uuid.New(), uuid.New()
	total, _ := appendN(t, sp, a, 2)
	require.NoError(t, sp.End(a, "closed", nil))
	require.NoError(t, sp.Close(a))
	require.NoError(t, sp.End(b, "closed", nil))
	runSpool(t, sp)

	require.Eventually(t, func() bool {
		return up.count(fmt.Sprintf("end %s closed", a)) == 1 && up.count(fmt.Sprintf("end %s closed", b)) == 1
	}, 5*time.Second, 5*time.Millisecond)
	require.Empty(t, sp.PendingEnds())
	time.Sleep(50 * time.Millisecond)
	require.FileExists(t, filepath.Join(dir, a.String(), "00000000.chunk"))
	require.FileExists(t, filepath.Join(dir, a.String(), "00000001.chunk"))
	require.Equal(t, total, sp.Total())
	require.Empty(t, up.finishes)

	up.setChunkCode(nil)
	require.Eventually(t, func() bool { return up.count(fmt.Sprintf("finish %s 2", a)) == 1 }, 5*time.Second, 5*time.Millisecond)
	require.Equal(t, 1, up.count(fmt.Sprintf("chunk %s 0", a)))
	require.Equal(t, 1, up.count(fmt.Sprintf("chunk %s 1", a)))
	require.Eventually(t, gone(filepath.Join(dir, a.String())), 5*time.Second, 5*time.Millisecond)
	require.Eventually(t, gone(filepath.Join(dir, b.String())), 5*time.Second, 5*time.Millisecond)
	require.Zero(t, sp.Total())
}

func TestSpoolAlreadyExistsCountsAsUploaded(t *testing.T) {
	up := &fakeUploader{}
	up.setChunkCode(func(seq int) connect.Code {
		if seq == 0 {
			return connect.CodeAlreadyExists // uploaded before, the response was lost
		}
		return 0
	})
	sp := openSpool(t, t.TempDir(), up, 0)
	id := uuid.New()
	_, _ = appendN(t, sp, id, 2)
	require.NoError(t, sp.Close(id))
	runSpool(t, sp)
	want := []string{fmt.Sprintf("chunk %s 1", id), fmt.Sprintf("finish %s 2", id)}
	require.Eventually(t, func() bool { return slices.Equal(up.journal(), want) }, 5*time.Second, 5*time.Millisecond)
	require.Zero(t, sp.Total())
}

func TestSpoolRejectedMovesAside(t *testing.T) {
	dir := t.TempDir()
	up := &fakeUploader{}
	up.code.Store(uint32(connect.CodeFailedPrecondition))
	sp := openSpool(t, dir, up, 0)
	id := uuid.New()
	_, _ = appendN(t, sp, id, 1)
	require.NoError(t, sp.End(id, "closed", nil))
	require.NoError(t, sp.Close(id))
	runSpool(t, sp)

	require.Eventually(t, func() bool {
		_, err := os.Stat(filepath.Join(dir, "rejected", id.String(), "00000000.chunk"))
		return err == nil
	},
		5*time.Second, 5*time.Millisecond)
	require.NoDirExists(t, filepath.Join(dir, id.String()))
	time.Sleep(100 * time.Millisecond)
	require.EqualValues(t, 1, up.calls.Load(), "no call after the refusal")
	require.Zero(t, sp.Total())
	require.Empty(t, sp.PendingEnds())

	again := openSpool(t, dir, up, 0)
	require.Zero(t, again.Total(), "spool/rejected is not counted after a restart")
	require.Empty(t, again.PendingEnds())
}

// An End during a long retry pause (the server's store is down, the server is up)
// is delivered at once, not after the pause.
func TestSpoolEndCutsRetryPause(t *testing.T) {
	up := &fakeUploader{}
	up.setChunkCode(func(int) connect.Code { return connect.CodeUnavailable })
	sp, err := recorder.OpenSpool(t.TempDir(), up, recorder.SpoolOptions{RetryMin: 5 * time.Second, RetryMax: 10 * time.Second},
		slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	a, b := uuid.New(), uuid.New()
	_, _ = appendN(t, sp, a, 1)
	runSpool(t, sp)
	require.Eventually(t, func() bool { return up.calls.Load() >= 1 }, 2*time.Second, time.Millisecond, "the first pass failed")

	require.NoError(t, sp.End(b, "closed", nil))
	require.Eventually(t, func() bool { return up.count(fmt.Sprintf("end %s closed", b)) == 1 }, 200*time.Millisecond, time.Millisecond)
	_, _ = appendN(t, sp, uuid.New(), 1) // new chunks do not cut the pause
	time.Sleep(50 * time.Millisecond)
	require.EqualValues(t, 3, up.calls.Load(), "chunk a, end b, chunk a")
}

// After a refusal, later chunks of the session fail at once, so the recorder ends
// it; no new session directory appears. Its end still goes out, and a second
// refusal keeps the first rejected copy.
func TestSpoolRejectedSessionRefusesAppend(t *testing.T) {
	dir := t.TempDir()
	up := &fakeUploader{}
	up.code.Store(uint32(connect.CodeInvalidArgument))
	sp := openSpool(t, dir, up, 0)
	runSpool(t, sp)
	id := uuid.New()
	_, _ = appendN(t, sp, id, 1)
	require.Eventually(t, func() bool { return sp.Pending(id) == 0 }, 5*time.Second, 5*time.Millisecond)
	require.DirExists(t, filepath.Join(dir, "rejected", id.String()))

	require.Error(t, sp.Append(id, 1, make([]byte, 32), []byte("gz")), "the session goes on")
	require.NoDirExists(t, filepath.Join(dir, id.String()))
	require.Zero(t, sp.Total())
	require.NoError(t, sp.End(id, "recording_unavailable", nil))
	require.Eventually(t, func() bool {
		entries, _ := os.ReadDir(filepath.Join(dir, "rejected"))
		return len(entries) == 2
	}, 5*time.Second, 5*time.Millisecond)
}

// A session the server keeps failing with a code that is neither a refusal nor
// an outage (Internal, Unimplemented) does not hold back the sessions after it.
func TestSpoolFailingSessionDoesNotBlockOthers(t *testing.T) {
	dir := t.TempDir()
	up := &fakeUploader{}
	a, b := uuid.MustParse("00000000-0000-4000-8000-000000000001"), uuid.MustParse("00000000-0000-4000-8000-000000000002")
	up.setChunkCode(func(seq int) connect.Code {
		if seq == 5 { // only a starts at seq 5
			return connect.CodeInternal
		}
		return 0
	})
	sp := openSpool(t, dir, up, 0)
	require.NoError(t, sp.Append(a, 5, make([]byte, 32), []byte("gz")))
	require.NoError(t, sp.Close(a))
	_, _ = appendN(t, sp, b, 1)
	require.NoError(t, sp.Close(b))
	runSpool(t, sp)

	require.Eventually(t, func() bool { return up.count(fmt.Sprintf("finish %s 1", b)) == 1 }, 5*time.Second, 5*time.Millisecond)
	require.FileExists(t, filepath.Join(dir, a.String(), "00000005.chunk"), "a is kept for a retry")
	require.NoDirExists(t, filepath.Join(dir, "rejected"))
	require.EqualValues(t, 2, sp.Total())
}
