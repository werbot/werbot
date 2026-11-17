package telnetconnector_test

import (
	"bytes"
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func tcpPair(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		c, _ := l.Accept()
		accepted <- c
	}()
	user, err := net.Dial("tcp", l.Addr().String())
	require.NoError(t, err)
	conn := <-accepted
	require.NotNil(t, conn)
	return user, conn
}

// spyRec keeps what the recorder received.
type spyRec struct {
	mu      sync.Mutex
	out, in bytes.Buffer
	resizes [][2]int

	panicInput atomic.Bool // Input panics
}

func (r *spyRec) Output(p []byte) { r.mu.Lock(); r.out.Write(p); r.mu.Unlock() }
func (r *spyRec) Input(p []byte) {
	if r.panicInput.Load() {
		panic("fault injected")
	}
	r.mu.Lock()
	r.in.Write(p)
	r.mu.Unlock()
}
func (r *spyRec) Resize(c, w int) {
	r.mu.Lock()
	r.resizes = append(r.resizes, [2]int{c, w})
	r.mu.Unlock()
}
func (*spyRec) Close(context.Context) error { return nil }

func (r *spyRec) output() string { r.mu.Lock(); defer r.mu.Unlock(); return r.out.String() }
func (r *spyRec) input() string  { r.mu.Lock(); defer r.mu.Unlock(); return r.in.String() }
func (r *spyRec) sizes() [][2]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([][2]int(nil), r.resizes...)
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

// scripted starts a target that runs script on its first connection; the
// script must return once the connection closes.
func scripted(t *testing.T, script func(c net.Conn)) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := l.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		script(c)
	}()
	t.Cleanup(func() { _ = l.Close(); <-done })
	return l.Addr().String()
}

// skipLine reads up to and including the next LF.
func skipLine(c net.Conn) error {
	b := make([]byte, 1)
	for {
		if _, err := c.Read(b); err != nil {
			return err
		}
		if b[0] == '\n' {
			return nil
		}
	}
}
