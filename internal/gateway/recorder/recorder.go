// Package recorder writes a session's terminal stream as asciicast v2,
// cut into gzip chunks linked by the recording hash chain.
package recorder

import (
	"bytes"
	"compress/gzip"
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/platform/hashchain"
)

// Recording limits and the session end reason for a lost recording.
const (
	MaxChunkRaw                   = 64 << 10 // uncompressed bytes per chunk
	MaxMarkerBytes                = 1024     // marker text cut to this on a UTF-8 boundary
	FlushInterval                 = 10 * time.Second
	EndReasonRecordingUnavailable = "recording_unavailable"
)

// ChunkSink stores chunks; *Spool implements it.
type ChunkSink interface {
	// Append must not retain gz after return: its buffer goes back to a pool.
	Append(sessionID uuid.UUID, seq int, hash, gz []byte) error
	Close(sessionID uuid.UUID) error
}

// Options tune a Recorder.
type Options struct {
	Terminate func(sessionID uuid.UUID, reason string) // nil → no-op
	Logger    *slog.Logger                             // nil → slog.Default()
}

// stream is one direction's unfinished UTF-8 tail, carried to its next write.
type stream struct {
	code byte
	tail [utf8.UTFMax]byte
	n    int
}

// Recorder implements gateway.RecordingSink. An idle session holds only the
// header and the chain state; the chunk buffer comes from a pool on demand.
type Recorder struct {
	mu        sync.Mutex
	id        uuid.UUID
	sink      ChunkSink
	terminate func(uuid.UUID, string)
	log       *slog.Logger
	start     time.Time
	header    []byte // written at the head of chunk seq 0
	out, in   stream
	buf       *bytes.Buffer // nil while no data is pending
	timer     *time.Timer
	seq       int
	prev      []byte
	failed    bool // a chunk was lost: data is dropped from now on
	notified  bool // Terminate was called
	closed    bool
}

// New starts a recording; cols or rows ≤ 0 (exec, stream) default to 80×24.
func New(sessionID uuid.UUID, cols, rows int, sink ChunkSink, opts Options) *Recorder {
	if opts.Terminate == nil {
		opts.Terminate = func(uuid.UUID, string) {}
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	start := time.Now()
	return &Recorder{
		id:        sessionID,
		sink:      sink,
		terminate: opts.Terminate,
		log:       opts.Logger,
		start:     start,
		header:    castHeader(cols, rows, start.Unix()),
		out:       stream{code: 'o'},
		in:        stream{code: 'i'},
		prev:      hashchain.Genesis(),
	}
}

// Output records target output.
func (r *Recorder) Output(p []byte) { r.write(&r.out, p) }

// Input records user input.
func (r *Recorder) Input(p []byte) { r.write(&r.in, p) }

// Resize records a window change.
func (r *Recorder) Resize(cols, rows int) {
	r.mu.Lock()
	defer r.unlock()
	var b [40]byte
	d := strconv.AppendInt(b[:0], int64(cols), 10)
	d = append(d, 'x')
	d = strconv.AppendInt(d, int64(rows), 10)
	r.event('r', d)
}

// Marker records a console line as an "m" event, cut to MaxMarkerBytes.
// Pending UTF-8 tails of o/i stay held, so a split rune survives a marker.
func (r *Recorder) Marker(text string) {
	if len(text) > MaxMarkerBytes {
		n := MaxMarkerBytes
		for n > 0 && !utf8.RuneStart(text[n]) {
			n--
		}
		text = text[:n]
	}
	r.mu.Lock()
	defer r.unlock()
	r.event('m', []byte(text))
}

// Close flushes the rest and closes the sink session; it is idempotent.
// After a sink failure it skips the flush but still closes the sink and returns nil.
func (r *Recorder) Close(context.Context) error {
	r.mu.Lock()
	defer r.unlock()
	if r.closed {
		return nil
	}
	for _, s := range []*stream{&r.out, &r.in} {
		if s.n > 0 {
			r.event(s.code, s.tail[:s.n]) // json.Marshal turns it into �
			s.n = 0
		}
	}
	if r.seq == 0 && !r.failed {
		r.ensureBuf() // an empty session still gets its header
	}
	r.flushLocked() // also stops the timer
	r.closed = true
	err := r.sink.Close(r.id)
	if r.failed {
		return nil
	}
	return err
}

// write emits p minus an unfinished UTF-8 tail, which waits for the stream's next write.
func (r *Recorder) write(s *stream, p []byte) {
	r.mu.Lock()
	defer r.unlock()
	if r.closed || r.failed {
		return
	}
	if s.n > 0 {
		p = append(s.tail[:s.n:s.n], p...)
		s.n = 0
	}
	k := incompleteTail(p)
	s.n = copy(s.tail[:], p[len(p)-k:])
	p = p[:len(p)-k]
	for len(p) > 0 && !r.failed {
		n := pieceLen(p)
		r.event(s.code, p[:n])
		p = p[n:]
	}
}

// event appends one line, flushing first when the chunk would exceed MaxChunkRaw.
func (r *Recorder) event(code byte, data []byte) {
	if r.closed || r.failed {
		return
	}
	var b [32]byte
	ts := strconv.AppendFloat(b[:0], time.Since(r.start).Seconds(), 'f', 6, 64)
	js := encodeData(data)
	if r.buf != nil && r.buf.Len()+eventLen(ts, js) > MaxChunkRaw {
		r.flushLocked()
		if r.failed {
			return
		}
	}
	r.ensureBuf()
	appendEvent(r.buf, ts, code, js)
}

// ensureBuf takes a chunk buffer from the pool and arms the flush timer.
func (r *Recorder) ensureBuf() {
	if r.buf != nil {
		return
	}
	r.buf = rawBuffers.Get().(*bytes.Buffer)
	if r.seq == 0 {
		r.buf.Write(r.header)
	}
	if r.timer == nil {
		r.timer = time.AfterFunc(FlushInterval, r.onTimer)
	} else {
		r.timer.Reset(FlushInterval)
	}
}

func (r *Recorder) onTimer() {
	r.mu.Lock()
	defer r.unlock()
	if !r.closed {
		r.flushLocked()
	}
}

// flushLocked compresses the pending buffer into chunk r.seq and hands it to
// the sink; it runs under r.mu. A sink error drops the chunk and marks the
// recording failed.
func (r *Recorder) flushLocked() {
	if r.timer != nil {
		r.timer.Stop()
	}
	raw := r.buf
	if raw == nil {
		return
	}
	r.buf = nil
	out := gzBuffers.Get().(*bytes.Buffer)
	zw := gzipWriters.Get().(*gzip.Writer)
	zw.Reset(out)                // zero header: same input → same gz
	_, _ = zw.Write(raw.Bytes()) // a bytes.Buffer never fails
	_ = zw.Close()
	gzipWriters.Put(zw)
	hash := hashchain.Next(r.prev, out.Bytes())
	err := r.sink.Append(r.id, r.seq, hash, out.Bytes())
	out.Reset()
	gzBuffers.Put(out)
	raw.Reset()
	rawBuffers.Put(raw)
	if err != nil {
		r.log.Error("recorder: chunk append failed", "session_id", r.id, "seq", r.seq, "error", err)
		r.failed = true
		return
	}
	r.prev = hash
	r.seq++
}

// unlock releases r.mu, then calls Terminate once after a failure, outside the lock.
func (r *Recorder) unlock() {
	notify := r.failed && !r.notified
	r.notified = r.notified || notify
	r.mu.Unlock()
	if notify {
		r.terminate(r.id, EndReasonRecordingUnavailable)
	}
}
