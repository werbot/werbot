package telnetconnector

import (
	"context"
	"io"

	"github.com/werbot/werbot/internal/gateway"
)

// relay copies the target to out (the recorded, redacted user output) and the
// user to the target, recording the input, and sends window changes other than
// the current one (initial) to the target as NAWS. The session ends when either
// side ends (the user's end closes the target). The input copy may outlive
// relay (a terminal read returns only when the front closes it), so its
// recording stops when relay returns. Both goroutines run under g.
func relay(ctx context.Context, g *gateway.Guard, tc *Conn, in io.Reader, out io.Writer,
	rec gateway.RecordingSink, resize <-chan gateway.Window, initial gateway.Window) error {
	live := gateway.NewLiveSink(rec)
	defer live.End()
	g.Detach(func() error {
		_, _ = io.Copy(tc, io.TeeReader(in, recordFunc(live.Input)))
		_ = tc.Close()
		return nil
	})
	g.Go(func() error {
		last := initial
		for {
			select {
			case w, ok := <-resize:
				if !ok {
					return nil
				}
				if w == last {
					continue
				}
				last = w
				_ = tc.SetWindowSize(w.Cols, w.Rows) // fails only once the target is gone
				live.Resize(w.Cols, w.Rows)
			case <-ctx.Done():
				return nil
			}
		}
	})

	_, err := io.Copy(out, tc)
	if gateway.IsClosed(err) {
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
