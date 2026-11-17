package connectortest

import (
	"context"
	"io"

	"github.com/werbot/werbot/internal/gateway"
)

// Echo is the test connector "echo": it sends a stream back to its sender.
// It records nothing, like a type whose Recorded lacks KindStream.
type Echo struct{}

// Type is "echo".
func (Echo) Type() string { return "echo" }

// Kinds is stream only.
func (Echo) Kinds() []gateway.Kind { return []gateway.Kind{gateway.KindStream} }

// Serve echoes s.Stream until EOF or ctx ends, then closes the stream.
func (Echo) Serve(ctx context.Context, s *gateway.Session) error {
	defer s.Stream.Close()
	stop := context.AfterFunc(ctx, func() { _ = s.Stream.Close() })
	defer stop()
	_, err := io.Copy(s.Stream, s.Stream)
	if ctx.Err() != nil {
		return nil // the stream was closed by the cancel
	}
	return err
}
