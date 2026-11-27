package recorder

import (
	"encoding/json"
	"errors"
)

// ErrFormat is the error of Event on an asciicast recording.
var ErrFormat = errors.New("recorder: event on asciicast recording")

// errUnavailable is the error of Event on a closed or lost recording.
var errUnavailable = errors.New("recorder: recording closed or lost")

// Event records v as one querylog line: its JSON and "\n". A line that fits in
// a chunk never spans two; a longer one fills chunks of MaxChunkRaw. An error
// means the line is not recorded: an asciicast recording (ErrFormat), a value
// that does not marshal, a closed recording, or a lost chunk.
func (r *Recorder) Event(v any) error {
	if !r.querylog {
		return ErrFormat
	}
	line, err := json.Marshal(v)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	r.mu.Lock()
	defer r.unlock()
	return r.line(line)
}

// line appends p under r.mu, flushing every full chunk on the way.
func (r *Recorder) line(p []byte) error {
	if r.closed || r.failed {
		return errUnavailable
	}
	if r.buf != nil && len(p) <= MaxChunkRaw && r.buf.Len()+len(p) > MaxChunkRaw {
		r.flushLocked() // the line starts the next chunk
	}
	for len(p) > 0 && !r.failed {
		r.ensureBuf()
		n := min(len(p), MaxChunkRaw-r.buf.Len())
		r.buf.Write(p[:n])
		p = p[n:]
		if r.buf.Len() == MaxChunkRaw {
			r.flushLocked()
		}
	}
	if r.failed {
		return errUnavailable
	}
	return nil
}
