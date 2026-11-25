package relay

import (
	"errors"
	"fmt"
	"sync"

	relayv1 "github.com/werbot/werbot/gen/werbot/relay/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
)

// Close reasons a stream carries: the gateway's token, passed through to the
// browser unchanged.
const (
	ReasonClosed   = "closed"
	ReasonDenied   = "denied"
	ReasonNotFound = "not_found"
	ReasonLimit    = "limit"
	ReasonError    = "error"
)

// Stream limits: one received message, one sent data frame and the window
// bounds a resize must keep.
const (
	pipeReadMaxBytes = 64 << 10
	pipeFrameBytes   = 32 << 10
	maxCols, maxRows = 500, 200
)

// inWindow reports whether cols×rows is within the window bounds.
func inWindow(cols, rows int) bool {
	return cols >= 1 && cols <= maxCols && rows >= 1 && rows <= maxRows
}

var (
	// ErrLimit is a stream refused because the replica holds MaxStreams.
	ErrLimit = fmt.Errorf("relay: stream limit reached: %w", apperr.ErrUnavailable)
	// ErrAwaitTimeout is a browser stream no gateway opened within AwaitTimeout.
	ErrAwaitTimeout = fmt.Errorf("relay: gateway did not open the stream: %w", apperr.ErrUnavailable)
	// errNotRunning is a stream asked for before the replica registered or
	// after its shutdown started.
	errNotRunning = fmt.Errorf("relay: not running: %w", apperr.ErrUnavailable)

	errPipeClosed = errors.New("relay: stream closed")
)

// Pipe is one browser stream: Read returns data, or a window size when cols
// > 0; a close by the other side is an error with a CloseReason() string
// method. Close is idempotent.
type Pipe interface {
	Read() (data []byte, cols, rows int, err error)
	Write(p []byte) error
	Resize(cols, rows int) error
	Close(reason string) error
}

// CloseError is the Read error of a stream the other side closed with Reason.
type CloseError struct{ Reason string }

func (e CloseError) Error() string { return "stream closed: " + e.Reason }

// CloseReason is the reason the other side gave.
func (e CloseError) CloseReason() string { return e.Reason }

// reasonOf is the close reason in err, or ReasonError for a stream that
// ended without one.
func reasonOf(err error) string {
	var r interface{ CloseReason() string }
	if errors.As(err, &r) {
		return r.CloseReason()
	}
	return ReasonError
}

// framePipe is the Pipe of one RelayService.Pipe stream, either end; sends
// are serialized, a close frame is sent once and nothing after it.
type framePipe struct {
	recv   func() (*relayv1.PipeFrame, error)
	send   func(*relayv1.PipeFrame) error
	mu     sync.Mutex
	closed bool
}

// Read returns the next data, an in-bounds resize or the close; other
// frames are skipped.
func (p *framePipe) Read() ([]byte, int, int, error) {
	for {
		f, err := p.recv()
		if err != nil {
			return nil, 0, 0, err
		}
		switch f := f.GetFrame().(type) {
		case *relayv1.PipeFrame_Data:
			return f.Data, 0, 0, nil
		case *relayv1.PipeFrame_Resize:
			if c, r := int(f.Resize.GetCols()), int(f.Resize.GetRows()); inWindow(c, r) {
				return nil, c, r, nil
			}
		case *relayv1.PipeFrame_Close:
			return nil, 0, 0, CloseError{Reason: f.Close}
		}
	}
}

// Write sends b in data frames of at most pipeFrameBytes.
func (p *framePipe) Write(b []byte) error {
	for len(b) > 0 {
		n := min(len(b), pipeFrameBytes)
		if err := p.put(&relayv1.PipeFrame{Frame: &relayv1.PipeFrame_Data{Data: b[:n]}}, false); err != nil {
			return err
		}
		b = b[n:]
	}
	return nil
}

func (p *framePipe) Resize(cols, rows int) error {
	return p.put(&relayv1.PipeFrame{Frame: &relayv1.PipeFrame_Resize{
		Resize: &relayv1.Resize{Cols: int32(cols), Rows: int32(rows)}}}, false) //nolint:gosec // Read bounds windows
}

func (p *framePipe) Close(reason string) error {
	err := p.put(&relayv1.PipeFrame{Frame: &relayv1.PipeFrame_Close{Close: reason}}, true)
	if errors.Is(err, errPipeClosed) {
		return nil
	}
	return err
}

// put sends f unless the pipe is closed; last marks it closed.
func (p *framePipe) put(f *relayv1.PipeFrame, last bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return errPipeClosed
	}
	p.closed = last
	return p.send(f)
}
