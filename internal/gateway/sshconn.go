package gateway

import (
	"net"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

// handshakeKey holds the *time.Timer that closes a connection still unauthenticated.
type handshakeKey struct{}

// handshakeConn is closed by its timer unless authentication stops it first;
// gliderlabs resets conn deadlines on every read, so a deadline cannot do this.
// Close stops the timer, so a failed or finished connection releases it at once.
type handshakeConn struct {
	net.Conn
	timer *time.Timer
}

func newHandshakeConn(ctx ssh.Context, conn net.Conn, timeout time.Duration) net.Conn {
	c := &handshakeConn{Conn: conn}
	c.timer = time.AfterFunc(timeout, func() { _ = conn.Close() })
	ctx.SetValue(handshakeKey{}, c.timer)
	return c
}

func (c *handshakeConn) Close() error {
	c.timer.Stop()
	return c.Conn.Close()
}

// authenticated stops the handshake timer once a signature is verified.
func authenticated(ctx ssh.Context) *gossh.ServerConfig {
	return &gossh.ServerConfig{VerifiedPublicKeyCallback: func(_ gossh.ConnMetadata, _ gossh.PublicKey, perms *gossh.Permissions, _ string) (*gossh.Permissions, error) {
		if t, ok := ctx.Value(handshakeKey{}).(*time.Timer); ok {
			t.Stop()
		}
		return perms, nil
	}}
}

// sessionChannel serves a session channel with gliderlabs, minus window changes
// sent before shell or exec. gliderlabs buffers one window (the pty-req one) and
// blocks its request loop on the next until our handler drains them; the handler
// starts only on shell or exec, so an early window change would hang the channel
// forever, even after the connection closes.
func sessionChannel(srv *ssh.Server, conn *gossh.ServerConn, nc gossh.NewChannel, ctx ssh.Context) {
	ssh.DefaultSessionHandler(srv, conn, earlyWindowFilter{nc}, ctx)
}

type earlyWindowFilter struct{ gossh.NewChannel }

// Accept relays the channel requests; until a shell or exec passes, a window
// change is acknowledged and dropped. The relay ends when the channel closes.
func (f earlyWindowFilter) Accept() (gossh.Channel, <-chan *gossh.Request, error) {
	ch, in, err := f.NewChannel.Accept()
	if err != nil {
		return ch, in, err
	}
	out := make(chan *gossh.Request)
	go func() {
		defer close(out)
		started := false
		for req := range in {
			if !started && req.Type == "window-change" {
				_ = req.Reply(true, nil) // no-op unless the client wants a reply
				continue
			}
			started = started || req.Type == "shell" || req.Type == "exec"
			out <- req
		}
	}()
	return ch, out, nil
}
