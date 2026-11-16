package connectortest

import (
	"context"
	"io"

	"github.com/werbot/werbot/internal/gateway"
)

// Echo is the test connector "echo": it sends a stream back to its sender.
type Echo struct{}

// Type is "echo".
func (Echo) Type() string { return "echo" }

// Kinds is stream only.
func (Echo) Kinds() []gateway.Kind { return []gateway.Kind{gateway.KindStream} }

// Serve echoes s.Stream through the recorder until EOF or ctx ends, then closes the stream.
func (Echo) Serve(ctx context.Context, s *gateway.Session) error {
	defer s.Stream.Close()
	stop := context.AfterFunc(ctx, func() { _ = s.Stream.Close() })
	defer stop()
	_, err := io.Copy(recWriter{s.Stream, s.Recording}, recReader{s.Stream, s.Recording})
	if ctx.Err() != nil {
		return nil // the stream was closed by the cancel
	}
	return err
}

type recReader struct {
	r   io.Reader
	rec gateway.RecordingSink
}

func (r recReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if n > 0 {
		r.rec.Input(p[:n])
	}
	return n, err
}

type recWriter struct {
	w   io.Writer
	rec gateway.RecordingSink
}

func (w recWriter) Write(p []byte) (int, error) {
	w.rec.Output(p)
	return w.w.Write(p)
}
