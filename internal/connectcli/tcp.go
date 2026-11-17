package connectcli

import (
	"context"
	"io"
	"net"
	"sync"

	"golang.org/x/crypto/ssh"
)

// serveTCP accepts local connections until the listener closes and forwards
// each one over its own direct-tcpip channel: one gateway session per
// connection. It returns after every forward ended.
func (b *binding) serveTCP(ctx context.Context) {
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		conn, err := b.ln.Accept()
		if err != nil {
			return
		}
		wg.Go(func() { b.forward(ctx, conn) })
	}
}

// forward relays conn and a direct-tcpip channel both ways. An end of input
// on one side is passed on as a half-close. Once the gateway's side ended, the
// forward ends when conn's side ended too or the gateway closed the channel;
// a dropped link or the end of ctx ends it at once.
func (b *binding) forward(ctx context.Context, conn net.Conn) {
	defer conn.Close() //nolint:errcheck // closed either way
	ch, reqs, err := b.openChannel(ctx, "direct-tcpip", conn.RemoteAddr())
	if err != nil {
		return
	}
	defer context.AfterFunc(ctx, func() { _ = conn.Close(); _ = ch.Close() })()
	var wg sync.WaitGroup
	defer wg.Wait()
	closed := make(chan struct{})
	wg.Go(func() { ssh.DiscardRequests(reqs); close(closed) }) // reqs ends when the channel closes
	wg.Go(func() { _, _ = io.Copy(b.stderr, ch.Stderr()) })
	up := make(chan struct{})
	go func() {
		defer close(up)
		_, _ = io.Copy(ch, conn)
		_ = ch.CloseWrite()
	}()
	_, _ = io.Copy(conn, ch) // the channel's data ends before its close
	if cw, ok := conn.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
	}
	select {
	case <-up:
	case <-closed:
	}
	_ = conn.Close()
	<-up
	_ = ch.Close()
}
