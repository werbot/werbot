package gateway

import (
	"context"
	"sync/atomic"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/gateway/recorder"
)

// RecordingSink receives one session's terminal stream, or its querylog lines.
type RecordingSink interface {
	Output(p []byte) // target output to the user
	Input(p []byte)  // user input
	Resize(cols, rows int)
	Marker(text string) // a console line as an asciicast "m" event
	Event(v any) error  // one querylog line, v as JSON; an asciicast recording refuses it
	Close(ctx context.Context) error
}

// RecordingFactory builds a sink per session; cols/rows are 0 for exec and
// stream; format is the resource type's RecordFormat.
type RecordingFactory func(sessionID uuid.UUID, cols, rows int, format string) RecordingSink

type nopSink struct{}

func (nopSink) Output([]byte)               {}
func (nopSink) Input([]byte)                {}
func (nopSink) Resize(int, int)             {}
func (nopSink) Marker(string)               {}
func (nopSink) Event(any) error             { return nil }
func (nopSink) Close(context.Context) error { return nil }

// NopRecording discards everything; the default until recording ships.
func NopRecording(uuid.UUID, int, int, string) RecordingSink { return nopSink{} }

// queryCounter counts the Event lines its sink accepted: the session's queries.
type queryCounter struct {
	RecordingSink
	n atomic.Int64
}

func (q *queryCounter) Event(v any) error {
	err := q.RecordingSink.Event(v)
	if err == nil {
		q.n.Add(1)
	}
	return err
}

// withQueries is r plus the accepted lines of a querylog recording rec; r
// stays as it is when rec counted none.
func withQueries(r *recorder.SessionResult, rec RecordingSink) *recorder.SessionResult {
	q, ok := rec.(*queryCounter)
	if !ok {
		return r
	}
	n := q.n.Load()
	if n == 0 {
		return r
	}
	if r == nil {
		r = &recorder.SessionResult{}
	}
	r.Queries = n
	return r
}
