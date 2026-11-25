package sshconnector

import (
	"context"
	"fmt"
	"net"

	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/gateway"
)

// serveRemoteForward listens on the bind address of rf at the target
// (tcpip-forward; an empty host is localhost), calls Ready and relays every
// connection to a channel rf.Open opens. A connection whose channel fails is
// dropped; the forward goes on. A cancel of ctx closes the listener
// (cancel-tcpip-forward) and the open connections and is not an error; a
// listener lost with the target connection is. Streams are not recorded.
func serveRemoteForward(ctx context.Context, client *gossh.Client, rf *gateway.RemoteForward, secret gateway.Secret) error {
	addr := rf.Addr()
	ln, err := client.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("ssh: remote forward %s at the target: %w", addr, err)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	g := gateway.NewGuard(cancel, secret) // a relay panic stops the forward and re-panics in Wait
	defer context.AfterFunc(ctx, func() { _ = ln.Close() })()
	rf.Ready()
	for {
		conn, err := ln.Accept()
		if err != nil {
			stopped := ctx.Err() != nil // the session ended, or a relay panicked
			cancel()
			_ = g.Wait() // the relays return nil; a panic re-panics here
			if stopped {
				return nil
			}
			return fmt.Errorf("ssh: remote forward %s ended: %w", addr, err)
		}
		g.Go(func() error { forwardConn(ctx, conn, rf, secret); return nil })
	}
}

// forwardConn relays conn from the target to a channel to the client until
// both directions end or ctx does.
func forwardConn(ctx context.Context, conn net.Conn, rf *gateway.RemoteForward, secret gateway.Secret) {
	origin, _ := conn.RemoteAddr().(*net.TCPAddr) // x/crypto parses it as one
	var host string
	var port uint32
	if origin != nil {
		host, port = origin.IP.String(), uint32(origin.Port) //nolint:gosec // a TCP port
	}
	ch, err := rf.Open(host, port)
	if err != nil {
		_ = conn.Close()
		return
	}
	defer context.AfterFunc(ctx, func() { _ = ch.Close(); _ = conn.Close() })()
	_ = pipe(ch, conn, secret) // one connection's failure is not the forward's
}
