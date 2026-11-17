package telnetconnector

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	telnetv1 "github.com/werbot/werbot/gen/werbot/connector/telnet/v1"
	"github.com/werbot/werbot/internal/gateway"
)

// stream serves a local telnet client: the gateway is its telnet server. The
// client's TERM and window (NAWS) are awaited for up to negotiateWait before
// the dial, so the target sees them before its login prompt; the recording
// uses the client window (80×24 without NAWS).
func stream(ctx context.Context, s *gateway.Session, g *gateway.Guard, cfg *telnetv1.Config) error {
	srv, err := NewServer(s.Stream)
	if err != nil {
		return fmt.Errorf("telnet: negotiate with the client: %w", err)
	}
	resize := make(chan gateway.Window, 1)
	srv.OnResize = func(cols, rows int) { // the only sender: after the drain the send never blocks
		select {
		case <-resize:
		default:
		}
		resize <- gateway.Window{Cols: cols, Rows: rows}
	}
	// The client is read from the start: negotiation is handled only while
	// reading. Serve closes the stream, which ends the copy.
	pr, pw := io.Pipe()
	defer pr.Close()
	g.Go(func() error {
		_, err := io.Copy(pw, srv)
		pw.CloseWithError(err)
		return nil
	})
	select {
	case <-srv.Negotiated():
	case <-time.After(negotiateWait):
	case <-ctx.Done():
		return nil
	}
	cols, rows := srv.Size()
	window := gateway.Window{Cols: cols, Rows: rows}

	tc, err := dial(ctx, s, cfg, cmp.Or(srv.TermType(), defaultTerm), window)
	if err != nil {
		return err
	}
	defer tc.Close()
	defer context.AfterFunc(ctx, func() { _ = tc.Close() })()
	if cols > 0 && rows > 0 {
		s.Recording.Resize(cols, rows)
	}
	pass := password(s, cfg.GetLogin())
	out := newRedactor(io.MultiWriter(recordFunc(s.Recording.Output), srv), pass)
	if err := autologin(ctx, tc, cfg.GetLogin(), pass, out); err != nil {
		if errors.Is(err, gateway.ErrLoginFailed) {
			_, _ = srv.Write([]byte("Login to " + s.Resource.Name + " failed\r\n"))
		}
		return err
	}
	return relay(ctx, g, tc, pr, out, s.Recording, resize, window)
}
