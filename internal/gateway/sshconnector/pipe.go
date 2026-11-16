package sshconnector

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/werbot/werbot/internal/gateway"
)

var bufPool = sync.Pool{New: func() any { b := make([]byte, 32<<10); return &b }}

// pipe relays a stream between the client and the target until both directions
// end. EOF on one side half-closes the other (CloseWrite, or Close when the
// side has none). Only the recorder sees the traffic: nothing is echoed.
func pipe(client, target io.ReadWriteCloser, rec gateway.RecordingSink) error {
	closeBoth := func() { _ = client.Close(); _ = target.Close() }
	g := &guard{stop: closeBoth}
	g.spawn(func() error { return relay(target, client, rec.Input) })
	g.spawn(func() error { return relay(client, target, rec.Output) })
	err := g.done()
	closeBoth()
	return err
}

// relay copies src to dst, then half-closes dst. A side closed by the other
// direction or by the session end is a normal end.
func relay(dst io.WriteCloser, src io.Reader, record func([]byte)) error {
	err := copyRec(dst, src, record)
	if errors.Is(err, net.ErrClosed) || errors.Is(err, io.ErrClosedPipe) {
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

// copyRec copies src to dst through a pooled 32 KB buffer and hands every chunk
// to record first. EOF is a nil error.
func copyRec(dst io.Writer, src io.Reader, record func([]byte)) error {
	bp := bufPool.Get().(*[]byte)
	defer bufPool.Put(bp)
	buf := *bp
	for {
		n, err := src.Read(buf)
		if n > 0 {
			record(buf[:n])
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return werr
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// guard runs the relay goroutines of one session. The pipeline recovers only
// the Serve goroutine, so a relay panic is recovered here: the first panic or
// error stops the session and becomes the result of done.
type guard struct {
	stop func()
	wg   sync.WaitGroup
	mu   sync.Mutex
	err  error
}

// spawn runs fn on a goroutine that done waits for.
func (g *guard) spawn(fn func() error) {
	g.wg.Go(func() { g.call(fn) })
}

// detach runs fn on a goroutine that done does not wait for: fn may block on
// a reader the session cannot interrupt, until the front closes it.
func (g *guard) detach(fn func() error) {
	go g.call(fn)
}

func (g *guard) call(fn func() error) {
	defer func() {
		if r := recover(); r != nil {
			g.fail(fmt.Errorf("ssh: relay panic: %v", r))
		}
	}()
	if err := fn(); err != nil {
		g.fail(err)
	}
}

func (g *guard) fail(err error) {
	g.mu.Lock()
	if g.err == nil {
		g.err = err
	}
	g.mu.Unlock()
	g.stop()
}

// done waits for the spawned goroutines and returns the first failure.
func (g *guard) done() error {
	g.wg.Wait()
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.err
}
