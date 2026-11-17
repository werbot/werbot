package telnetconnector

import (
	"errors"
	"io"
	"net"
	"sync"

	"github.com/werbot/werbot/internal/gateway"
)

// relay copies the target to the user and the user to the target, recording
// both, and sends window changes to the target as NAWS. The session ends when
// either side ends (the user's end closes the target). The input copy may
// outlive relay (a terminal read returns only when the front closes it), so its
// recording stops when relay returns.
func relay(tc *Conn, in io.Reader, out io.Writer, rec gateway.RecordingSink, resize <-chan gateway.Window) error {
	live := &liveInput{rec: rec}
	defer live.end()
	go func() {
		_, _ = io.Copy(tc, io.TeeReader(in, live))
		_ = tc.Close()
	}()
	ended := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			select {
			case w, ok := <-resize:
				if !ok {
					return
				}
				_ = tc.SetWindowSize(w.Cols, w.Rows) // fails only once the target is gone
				rec.Resize(w.Cols, w.Rows)
			case <-ended:
				return
			}
		}
	})
	defer wg.Wait()
	defer close(ended)

	_, err := io.Copy(out, io.TeeReader(tc, recordFunc(rec.Output)))
	if errors.Is(err, net.ErrClosed) || errors.Is(err, io.ErrClosedPipe) || errors.Is(err, io.EOF) {
		return nil // closed by the user's end or the cancel; an SSH channel write after close is io.EOF
	}
	return err
}

// recordFunc is a recorder method as an io.Writer.
type recordFunc func([]byte)

func (f recordFunc) Write(p []byte) (int, error) {
	f(p)
	return len(p), nil
}

// liveInput passes user input to the recorder only until relay returns: the
// pipeline closes the recording after Serve.
type liveInput struct {
	mu    sync.Mutex
	ended bool
	rec   gateway.RecordingSink
}

func (l *liveInput) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.ended {
		l.rec.Input(p)
	}
	return len(p), nil
}

func (l *liveInput) end() {
	l.mu.Lock()
	l.ended = true
	l.mu.Unlock()
}
