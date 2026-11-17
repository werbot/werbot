// Package tcpconnector is the gateway connector of the "tcp" resource type: a
// raw byte stream between the SSH front's direct-tcpip channel and a target
// port of the resource. Streams are counted, not recorded.
package tcpconnector

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/werbot/werbot/internal/gateway"
)

func init() { gateway.RegisterConnector(Connector{}) }

// Connector serves "tcp" sessions.
type Connector struct{}

// Type is "tcp".
func (Connector) Type() string { return "tcp" }

// Kinds is stream only.
func (Connector) Kinds() []gateway.Kind { return []gateway.Kind{gateway.KindStream} }

// DialsTarget marks the connector as dialing through Session.Dial.
func (Connector) DialsTarget() {}

// Serve dials the resource host at the port of s.Target and relays the stream
// both ways until both sides end. A cancel of ctx closes both and is not an
// error. A relay panic stops the session and re-panics here for the pipeline.
func (Connector) Serve(ctx context.Context, s *gateway.Session) error {
	defer s.Stream.Close()
	sctx, stop := context.WithCancel(ctx)
	defer stop()
	target, err := dial(sctx, s)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	closeBoth := func() { _ = s.Stream.Close(); _ = target.Close() }
	defer context.AfterFunc(sctx, closeBoth)()
	g := gateway.NewGuard(stop, s.Secret)
	g.Go(func() error { return relay(target, s.Stream) })
	g.Go(func() error { return relay(s.Stream, target) })
	err = g.Wait()
	closeBoth()
	if ctx.Err() != nil {
		return nil
	}
	return err
}

// dial connects to the resource host at the requested port through Session.Dial.
// The error carries no address: the target is not for the user's eyes.
func dial(ctx context.Context, s *gateway.Session) (net.Conn, error) {
	_, port, err := net.SplitHostPort(s.Target)
	if err != nil {
		return nil, gateway.ErrTargetNotAllowed
	}
	ep, _, err := s.Resource.Endpoint()
	if err != nil {
		return nil, fmt.Errorf("tcp: resource config: %w", err)
	}
	conn, err := s.Dial(ctx, "tcp", net.JoinHostPort(ep.Host, port))
	if err == nil {
		return conn, nil
	}
	if errors.Is(err, gateway.ErrTargetNotAllowed) {
		return nil, fmt.Errorf("tcp: dial target: %w", gateway.ErrTargetNotAllowed)
	}
	if oe, ok := errors.AsType[*net.OpError](err); ok {
		err = oe.Err
	}
	return nil, fmt.Errorf("tcp: dial target: %w", err)
}

// relay copies src to dst, then half-closes dst (Close when it has no CloseWrite).
// A side closed by the other direction or by the session end is a normal end.
func relay(dst io.WriteCloser, src io.Reader) error {
	_, err := gateway.CopyBuffer(dst, src)
	if gateway.IsClosed(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if cw, ok := dst.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
		return nil
	}
	_ = dst.Close()
	return nil
}
