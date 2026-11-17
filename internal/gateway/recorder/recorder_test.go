package recorder_test

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/recorder"
	"github.com/werbot/werbot/internal/platform/hashchain"
)

var _ gateway.RecordingSink = (*recorder.Recorder)(nil)

type chunk struct {
	seq      int
	hash, gz []byte
}

// memSink copies every chunk; Append number failFrom (1-based) and later fail.
type memSink struct {
	mu       sync.Mutex
	failFrom int
	calls    int
	chunks   []chunk
	closed   bool
}

func (s *memSink) Append(_ uuid.UUID, seq int, hash, gz []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.failFrom > 0 && s.calls >= s.failFrom {
		return errors.New("sink down")
	}
	s.chunks = append(s.chunks, chunk{seq, bytes.Clone(hash), bytes.Clone(gz)})
	return nil
}

func (s *memSink) Close(uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func (s *memSink) snapshot() []chunk {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]chunk(nil), s.chunks...)
}

func gunzip(t *testing.T, gz []byte) []byte {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	require.NoError(t, err)
	raw, err := io.ReadAll(zr)
	require.NoError(t, err)
	return raw
}

// cast gunzips all chunks in order.
func (s *memSink) cast(t *testing.T) []byte {
	t.Helper()
	var out []byte
	for _, c := range s.snapshot() {
		out = append(out, gunzip(t, c.gz)...)
	}
	return out
}

func assertChain(t *testing.T, s *memSink) {
	t.Helper()
	prev := hashchain.Genesis()
	for i, c := range s.snapshot() {
		assert.Equal(t, i, c.seq)
		prev = hashchain.Next(prev, c.gz)
		assert.Equal(t, prev, c.hash, "chunk %d", i)
	}
}

type event struct {
	code, data string
}

// assertCastParses checks the header and every event line, returning the events.
func assertCastParses(t *testing.T, cast []byte) []event {
	t.Helper()
	sc := bufio.NewScanner(bytes.NewReader(cast))
	sc.Buffer(nil, 1<<20)
	require.True(t, sc.Scan())
	var hdr map[string]any
	require.NoError(t, json.Unmarshal(sc.Bytes(), &hdr))
	assert.EqualValues(t, 2, hdr["version"])
	assert.Contains(t, hdr, "width")
	assert.Contains(t, hdr, "height")
	var events []event
	for sc.Scan() {
		var ev []any
		require.NoError(t, json.Unmarshal(sc.Bytes(), &ev), "line %q", sc.Text())
		require.Len(t, ev, 3)
		assert.IsType(t, float64(0), ev[0])
		code, _ := ev[1].(string)
		assert.Contains(t, []string{"o", "i", "r"}, code)
		data, ok := ev[2].(string)
		require.True(t, ok)
		events = append(events, event{code, data})
	}
	require.NoError(t, sc.Err())
	return events
}

// output concatenates the data of all "o" events.
func output(events []event) string {
	var b strings.Builder
	for _, e := range events {
		if e.code == "o" {
			b.WriteString(e.data)
		}
	}
	return b.String()
}

func TestRecorderGolden(t *testing.T) {
	synctest.Test(t, func(t *testing.T) { // fake clock starts at 2000-01-01T00:00:00Z
		sink := &memSink{}
		r := recorder.New(uuid.Must(uuid.NewV7()), 80, 24, sink, recorder.Options{})
		time.Sleep(1500 * time.Millisecond)
		r.Output([]byte("hello\r\n"))
		r.Input([]byte("ls\r"))
		time.Sleep(250 * time.Millisecond)
		r.Resize(100, 30)
		time.Sleep(250 * time.Millisecond)
		b := []byte("привет")
		r.Output(b[:5]) // split inside «и»
		time.Sleep(100 * time.Millisecond)
		r.Output(b[5:])
		require.NoError(t, r.Close(context.Background()))
		want, err := os.ReadFile("testdata/basic.cast")
		require.NoError(t, err)
		assert.Equal(t, string(want), string(sink.cast(t)))
		assertCastParses(t, sink.cast(t))
		assert.True(t, sink.closed)
		assertChain(t, sink)
	})
}

func TestRecorderEmptySessionWritesHeader(t *testing.T) {
	sink := &memSink{}
	r := recorder.New(uuid.Must(uuid.NewV7()), 0, 0, sink, recorder.Options{})
	require.NoError(t, r.Close(context.Background()))
	require.NoError(t, r.Close(context.Background())) // idempotent
	require.Len(t, sink.snapshot(), 1)
	assert.Contains(t, string(sink.cast(t)), `{"version":2,"width":80,"height":24,`)
	assert.Empty(t, assertCastParses(t, sink.cast(t)))
	assertChain(t, sink)
}

func TestRecorderInvalidUTF8(t *testing.T) {
	sink := &memSink{}
	r := recorder.New(uuid.Must(uuid.NewV7()), 80, 24, sink, recorder.Options{})
	r.Output([]byte{0xff, 'a'})
	r.Output([]byte{0xd0})
	require.NoError(t, r.Close(context.Background()))
	events := assertCastParses(t, sink.cast(t))
	assert.Equal(t, []event{{"o", "�a"}, {"o", "�"}}, events)
}

func TestRecorderSplitsLargeOutput(t *testing.T) {
	for name, b := range map[string]byte{
		"ascii":        'a',
		"worst escape": 0x01, // JSON \u0001: ×6
	} {
		t.Run(name, func(t *testing.T) {
			sink := &memSink{}
			r := recorder.New(uuid.Must(uuid.NewV7()), 80, 24, sink, recorder.Options{})
			in := bytes.Repeat([]byte{b}, 1<<20)
			r.Output(in)
			require.NoError(t, r.Close(context.Background()))
			chunks := sink.snapshot()
			assert.GreaterOrEqual(t, len(chunks), 16)
			for i, c := range chunks {
				assert.LessOrEqual(t, len(gunzip(t, c.gz)), recorder.MaxChunkRaw, "chunk %d", i)
			}
			assert.Equal(t, string(in), output(assertCastParses(t, sink.cast(t))))
			assertChain(t, sink)
		})
	}
}

func TestRecorderKeepsSplitEmoji(t *testing.T) {
	emoji := []byte("😀")
	for cut := 1; cut < len(emoji); cut++ {
		sink := &memSink{}
		r := recorder.New(uuid.Must(uuid.NewV7()), 80, 24, sink, recorder.Options{})
		r.Output(append([]byte("a"), emoji[:cut]...))
		r.Output(append(emoji[cut:], 'b'))
		require.NoError(t, r.Close(context.Background()))
		events := assertCastParses(t, sink.cast(t))
		assert.Equal(t, "a😀b", output(events), "cut %d", cut)
	}
}

func TestRecorderFlushesAfterInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sink := &memSink{}
		r := recorder.New(uuid.Must(uuid.NewV7()), 80, 24, sink, recorder.Options{})
		r.Output([]byte("x"))
		time.Sleep(recorder.FlushInterval - time.Millisecond)
		synctest.Wait()
		assert.Empty(t, sink.snapshot())
		time.Sleep(time.Millisecond)
		synctest.Wait()
		assert.Len(t, sink.snapshot(), 1)
		require.NoError(t, r.Close(context.Background()))
	})
}

func TestRecorderTerminatesWhenSinkFails(t *testing.T) {
	sink := &memSink{failFrom: 2}
	id := uuid.Must(uuid.NewV7())
	type call struct {
		id     uuid.UUID
		reason string
	}
	var calls []call
	r := recorder.New(id, 80, 24, sink, recorder.Options{
		Terminate: func(sid uuid.UUID, reason string) { calls = append(calls, call{sid, reason}) },
	})
	in := make([]byte, 200<<10)
	_, _ = rand.Read(in)
	r.Output(in)
	require.Equal(t, []call{{id, recorder.EndReasonRecordingUnavailable}}, calls)
	appends := sink.calls
	r.Output([]byte("more"))
	require.NoError(t, r.Close(context.Background()))
	assert.Equal(t, appends, sink.calls)
	assert.Len(t, calls, 1)
	assert.True(t, sink.closed)
}

func TestRecorderConcurrentSessions(t *testing.T) {
	sinks := make([]*memSink, 8)
	inputs := make([][]byte, 8)
	var wg sync.WaitGroup
	for i := range sinks {
		sinks[i] = &memSink{}
		inputs[i] = bytes.Repeat([]byte{byte('A' + i)}, 300<<10)
		wg.Go(func() {
			r := recorder.New(uuid.Must(uuid.NewV7()), 80, 24, sinks[i], recorder.Options{})
			r.Output(inputs[i])
			assert.NoError(t, r.Close(context.Background()))
		})
	}
	wg.Wait()
	for i, s := range sinks {
		assert.Equal(t, string(inputs[i]), output(assertCastParses(t, s.cast(t))), "session %d", i)
		assertChain(t, s)
	}
}
