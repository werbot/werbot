package sshconnector

import (
	"io"

	"github.com/werbot/werbot/internal/gateway"
)

// pipe relays a stream between the client and the target until both directions
// end. EOF on one side half-closes the other (CloseWrite, or Close when the
// side has none). Streams are not recorded (ssh records terminal and exec only);
// nothing is echoed.
func pipe(client, target io.ReadWriteCloser, secret gateway.Secret) error {
	closeBoth := func() { _ = client.Close(); _ = target.Close() }
	g := gateway.NewGuard(closeBoth, secret)
	g.Go(func() error { return relay(target, client) })
	g.Go(func() error { return relay(client, target) })
	err := g.Wait()
	closeBoth()
	return err
}

// relay copies src to dst, then half-closes dst. A side closed by the other
// direction or by the session end is a normal end (an SSH channel write after
// the remote close is io.EOF).
func relay(dst io.WriteCloser, src io.Reader) error {
	err := copyRec(dst, src, nil)
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

// copyRec copies src to dst and hands every chunk to record first (nil: none).
// EOF is a nil error.
func copyRec(dst io.Writer, src io.Reader, record func([]byte)) error {
	if record != nil {
		dst = recordWriter{dst, record}
	}
	_, err := gateway.CopyBuffer(dst, src)
	return err
}

// recordWriter hands every chunk to record before it writes it.
type recordWriter struct {
	io.Writer
	record func([]byte)
}

func (w recordWriter) Write(p []byte) (int, error) {
	w.record(p)
	return w.Writer.Write(p)
}
