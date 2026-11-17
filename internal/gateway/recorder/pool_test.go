package recorder

import (
	"bytes"
	"context"
	"runtime"
	"runtime/debug"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type discardSink struct{ appends int }

func (s *discardSink) Append(uuid.UUID, int, []byte, []byte) error { s.appends++; return nil }
func (s *discardSink) Close(uuid.UUID) error                       { return nil }

// hasBuf reads r.buf under r.mu: the flush timer writes it from its own goroutine.
func hasBuf(r *Recorder) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf != nil
}

func TestRecorderIdleHoldsNoBuffer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sink := &discardSink{}
		r := New(uuid.Must(uuid.NewV7()), 80, 24, sink, Options{})
		assert.False(t, hasBuf(r))
		r.Output([]byte("x"))
		assert.True(t, hasBuf(r))
		time.Sleep(FlushInterval)
		synctest.Wait()
		assert.False(t, hasBuf(r)) // takes r.mu: orders the timer's Append before the read
		assert.Equal(t, 1, sink.appends)
		r.Output([]byte("y"))
		assert.True(t, hasBuf(r))
		require.NoError(t, r.Close(context.Background()))
		assert.False(t, hasBuf(r))
		assert.Equal(t, 2, sink.appends)
	})
}

func TestRecorderFlushAllocs(t *testing.T) {
	if raceEnabled {
		t.Skip("sync.Pool drops objects under -race")
	}
	r := New(uuid.Must(uuid.NewV7()), 80, 24, &discardSink{}, Options{})
	data := bytes.Repeat([]byte("a"), 1<<10)
	cycle := func() {
		r.Output(data)
		r.mu.Lock()
		r.flushLocked()
		r.mu.Unlock()
	}
	runtime.GC() // a GC empties sync.Pool; start the warm-up after it
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	for range 3 {
		cycle()
	}
	allocs := testing.AllocsPerRun(100, cycle)
	assert.LessOrEqual(t, allocs, 10.0)

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for range 100 {
		cycle()
	}
	runtime.ReadMemStats(&after)
	perCycle := (after.TotalAlloc - before.TotalAlloc) / 100
	assert.Less(t, perCycle, uint64(32<<10), "gzip.NewWriter per chunk costs hundreds of KiB")
	t.Logf("allocs/cycle=%.0f bytes/cycle=%d", allocs, perCycle)
}
