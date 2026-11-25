package gateway

import (
	"context"

	"github.com/google/uuid"
)

// RecordingSink receives one session's terminal stream.
type RecordingSink interface {
	Output(p []byte) // target output to the user
	Input(p []byte)  // user input
	Resize(cols, rows int)
	Marker(text string) // a console line as an asciicast "m" event
	Close(ctx context.Context) error
}

// RecordingFactory builds a sink per session; cols/rows are 0 for exec and stream.
type RecordingFactory func(sessionID uuid.UUID, cols, rows int) RecordingSink

type nopSink struct{}

func (nopSink) Output([]byte)               {}
func (nopSink) Input([]byte)                {}
func (nopSink) Resize(int, int)             {}
func (nopSink) Marker(string)               {}
func (nopSink) Close(context.Context) error { return nil }

// NopRecording discards everything; the default until recording ships.
func NopRecording(uuid.UUID, int, int) RecordingSink { return nopSink{} }
