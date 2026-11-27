package recorder_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway/recorder"
)

// maxChunkGz is session.MaxChunkGz, the server's bound on a compressed chunk.
const maxChunkGz = 128 << 10

func newQuerylog(sink *memSink) *recorder.Recorder {
	return recorder.New(uuid.Must(uuid.NewV7()), 0, 0, sink, recorder.Options{Format: "querylog"})
}

// A querylog recording is one JSON line per Event, without a header, through
// the same hash chain.
func TestRecorderQuerylog(t *testing.T) {
	sink := &memSink{}
	r := newQuerylog(sink)
	for range 3 {
		require.NoError(t, r.Event(map[string]any{"kind": "query"}))
	}
	require.NoError(t, r.Close(t.Context()))
	assert.Equal(t, strings.Repeat(`{"kind":"query"}`+"\n", 3), string(sink.cast(t)))
	assertChain(t, sink)
	assert.True(t, sink.closed)
}

// A line longer than a chunk spans chunks of at most MaxChunkRaw raw bytes;
// joined, they give the line back.
func TestRecorderQuerylogLongLine(t *testing.T) {
	sink := &memSink{}
	r := newQuerylog(sink)
	long := strings.Repeat("x", 300<<10)
	require.NoError(t, r.Event(map[string]string{"sql": long}))
	require.NoError(t, r.Close(t.Context()))
	chunks := sink.snapshot()
	require.GreaterOrEqual(t, len(chunks), 5)
	for _, c := range chunks {
		assert.LessOrEqual(t, len(gunzip(t, c.gz)), recorder.MaxChunkRaw)
		assert.LessOrEqual(t, len(c.gz), maxChunkGz)
	}
	var got map[string]string
	require.NoError(t, json.Unmarshal(sink.cast(t), &got))
	assert.True(t, got["sql"] == long, "the joined chunks are the line")
	assertChain(t, sink)
}

// A line that fits in a chunk starts a new chunk rather than spanning two.
func TestRecorderQuerylogLineInOneChunk(t *testing.T) {
	sink := &memSink{}
	r := newQuerylog(sink)
	text := strings.Repeat("y", 40<<10) // two lines do not fit in one chunk
	for range 3 {
		require.NoError(t, r.Event(map[string]string{"sql": text}))
	}
	require.NoError(t, r.Close(t.Context()))
	chunks := sink.snapshot()
	require.Len(t, chunks, 3)
	for i, c := range chunks {
		raw := gunzip(t, c.gz)
		require.Equal(t, 1, bytes.Count(raw, []byte("\n")), "chunk %d", i)
		require.True(t, json.Valid(raw), "chunk %d holds one whole line", i)
	}
	assertChain(t, sink)
}

// Event belongs to querylog: an asciicast recording refuses it; a querylog
// recording ignores the terminal stream.
func TestRecorderEventOnCast(t *testing.T) {
	cast := &memSink{}
	r := recorder.New(uuid.Must(uuid.NewV7()), 80, 24, cast, recorder.Options{})
	require.ErrorIs(t, r.Event(map[string]string{"kind": "query"}), recorder.ErrFormat)

	sink := &memSink{}
	q := newQuerylog(sink)
	q.Output([]byte("out"))
	q.Input([]byte("in"))
	q.Resize(100, 50)
	q.Marker("ls")
	require.NoError(t, q.Close(t.Context()))
	assert.Empty(t, sink.cast(t))
	assertChain(t, sink)
}

// Event fails once the recording is closed or lost: the line is not counted.
func TestRecorderQuerylogEventErrors(t *testing.T) {
	sink := &memSink{failFrom: 1}
	terminated := make(chan string, 1)
	r := recorder.New(uuid.Must(uuid.NewV7()), 0, 0, sink, recorder.Options{Format: "querylog",
		Terminate: func(_ uuid.UUID, reason string) { terminated <- reason }})
	line := map[string]string{"sql": strings.Repeat("z", 40<<10)}
	require.NoError(t, r.Event(line))
	require.Error(t, r.Event(line), "the chunk before it was lost")
	require.Equal(t, recorder.EndReasonRecordingUnavailable, <-terminated)
	require.Error(t, r.Event(line))
	require.Error(t, r.Event(func() {}), "a value that does not marshal")
	require.NoError(t, r.Close(t.Context()))

	closed := newQuerylog(&memSink{})
	require.NoError(t, closed.Close(t.Context()))
	require.Error(t, closed.Event(line))
}
