package recorder_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/gateway/recorder"
	"github.com/werbot/werbot/internal/platform/metrics"
)

// fakeUploader is the server side of a spool. Only successful calls go to the log.
type fakeUploader struct {
	down     atomic.Bool   // every call fails with Unavailable
	code     atomic.Uint32 // connect.Code every call fails with; 0: none
	calls    atomic.Int32  // every attempt
	endCalls atomic.Int32  // EndSession attempts

	mu        sync.Mutex
	chunkCode func(seq int) connect.Code // UploadChunk only; nil or 0: success
	log       []string
	finishes  []recorder.Manifest
}

func (u *fakeUploader) fail() error {
	u.calls.Add(1)
	if u.down.Load() {
		return connect.NewError(connect.CodeUnavailable, errors.New("down"))
	}
	if c := u.code.Load(); c != 0 {
		return connect.NewError(connect.Code(c), errors.New("refused"))
	}
	return nil
}

func (u *fakeUploader) note(s string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.log = append(u.log, s)
}

func (u *fakeUploader) UploadChunk(_ context.Context, id uuid.UUID, seq int, hash, gz []byte) error {
	if err := u.fail(); err != nil {
		return err
	}
	u.mu.Lock()
	code := connect.Code(0)
	if u.chunkCode != nil {
		code = u.chunkCode(seq)
	}
	u.mu.Unlock()
	if code != 0 {
		return connect.NewError(code, errors.New("chunk"))
	}
	if len(hash) != 32 || len(gz) == 0 {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("bad chunk"))
	}
	u.note(fmt.Sprintf("chunk %s %d", id, seq))
	return nil
}

func (u *fakeUploader) EndSession(_ context.Context, id uuid.UUID, reason string, c *gatewayv1.SessionCounters, r *recorder.SessionResult) error {
	u.endCalls.Add(1)
	if err := u.fail(); err != nil {
		return err
	}
	if c != nil {
		reason += fmt.Sprintf(" %s %d/%d/%d/%d", c.GetSessionId(), c.GetBytesUp(), c.GetBytesDown(), c.GetDatagramsUp(), c.GetDatagramsDown())
	}
	if r != nil {
		reason += fmt.Sprintf(" %q %v %v", r.Command, r.Classes, r.Paths)
	}
	u.note(fmt.Sprintf("end %s %s", id, reason))
	return nil
}

func (u *fakeUploader) FinishRecording(_ context.Context, m recorder.Manifest) error {
	if err := u.fail(); err != nil {
		return err
	}
	u.mu.Lock()
	u.finishes = append(u.finishes, m)
	u.mu.Unlock()
	u.note(fmt.Sprintf("finish %s %d", m.SessionID, m.Chunks))
	return nil
}

func (u *fakeUploader) setChunkCode(fn func(seq int) connect.Code) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.chunkCode = fn
}

func (u *fakeUploader) journal() []string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return slices.Clone(u.log)
}

func (u *fakeUploader) count(s string) int {
	n := 0
	for _, l := range u.journal() {
		if l == s {
			n++
		}
	}
	return n
}

func openSpool(t *testing.T, dir string, up recorder.Uploader, limit int64) *recorder.Spool {
	t.Helper()
	sp, err := recorder.OpenSpool(dir, up, recorder.SpoolOptions{Limit: limit, RetryMin: 10 * time.Millisecond, RetryMax: 50 * time.Millisecond},
		slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	return sp
}

// runSpool runs sp until the test ends.
func runSpool(t *testing.T, sp *recorder.Spool) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- sp.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-done)
	})
}

// appendN appends n chunks of random gz to id and returns their total size and the last hash.
func appendN(t *testing.T, sp *recorder.Spool, id uuid.UUID, n int) (int64, []byte) {
	t.Helper()
	var total int64
	var hash []byte
	for seq := range n {
		hash = bytes.Repeat([]byte{byte(seq + 1)}, 32)
		gz := make([]byte, 100+seq)
		_, _ = rand.Read(gz)
		require.NoError(t, sp.Append(id, seq, hash, gz))
		total += int64(len(gz))
	}
	return total, hash
}

func gone(dir string) func() bool {
	return func() bool { _, err := os.Stat(dir); return errors.Is(err, os.ErrNotExist) }
}

func TestDefaultSpoolLimit(t *testing.T) {
	assert.EqualValues(t, 67108864, recorder.DefaultSpoolLimit)
}

func TestSpoolLimit(t *testing.T) {
	reg := metrics.NewRegistry()
	sp, err := recorder.OpenSpool(t.TempDir(), &fakeUploader{}, recorder.SpoolOptions{Limit: 100, Gauge: reg.Gauge("spool_bytes", "test")},
		slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	a, b := uuid.New(), uuid.New()
	h := make([]byte, 32)
	require.NoError(t, sp.Append(a, 0, h, make([]byte, 60)))
	require.ErrorIs(t, sp.Append(a, 1, h, make([]byte, 60)), recorder.ErrSpoolFull)
	require.EqualValues(t, 60, sp.Pending(a))
	require.NoError(t, sp.Append(b, 0, h, make([]byte, 50)), "the limit is per session")
	require.EqualValues(t, 110, sp.Total())
	rec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Contains(t, rec.Body.String(), "spool_bytes 110")
}

func TestSpoolCapTerminatesRecorder(t *testing.T) {
	up := &fakeUploader{}
	up.down.Store(true)
	sp := openSpool(t, t.TempDir(), up, 64<<10)
	runSpool(t, sp)
	id := uuid.New()
	var mu sync.Mutex
	var reasons []string
	r := recorder.New(id, 80, 24, sp, recorder.Options{Terminate: func(got uuid.UUID, reason string) {
		mu.Lock()
		defer mu.Unlock()
		require.Equal(t, id, got)
		reasons = append(reasons, reason)
	}})
	in := make([]byte, 512<<10)
	_, _ = rand.Read(in)
	r.Output(in)
	require.NoError(t, r.Close(t.Context()))
	mu.Lock()
	require.Equal(t, []string{recorder.EndReasonRecordingUnavailable}, reasons)
	mu.Unlock()
	require.LessOrEqual(t, sp.Pending(id), int64(64<<10))
	require.Positive(t, sp.Pending(id))
}

// A shut down spool writes nothing: a new Spool owns the directory.
func TestSpoolShutdown(t *testing.T) {
	dir := t.TempDir()
	sp := openSpool(t, dir, &fakeUploader{}, 0)
	id := uuid.New()
	_, _ = appendN(t, sp, id, 1)
	require.NoError(t, sp.Shutdown())
	require.Error(t, sp.Append(id, 1, make([]byte, 32), []byte("gz")))
	require.Error(t, sp.End(id, "closed", nil, nil))
	require.Error(t, sp.Close(id))
	require.Error(t, sp.Append(uuid.New(), 0, make([]byte, 32), []byte("gz")))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.NoFileExists(t, filepath.Join(dir, id.String(), "end.json"))
	require.NoFileExists(t, filepath.Join(dir, id.String(), "00000001.chunk"))
}
