package gateway

import (
	"context"
	"encoding/binary"
	"net"
	"sync/atomic"
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

// authenticated stops the handshake timer once a signature is verified for a
// user. A connection the control plane could not check (no user) keeps the
// timer: every channel gets only the not ready text, so the limit closes it.
func authenticated(ctx ssh.Context) *gossh.ServerConfig {
	return &gossh.ServerConfig{VerifiedPublicKeyCallback: func(_ gossh.ConnMetadata, _ gossh.PublicKey, perms *gossh.Permissions, _ string) (*gossh.Permissions, error) {
		if t, ok := ctx.Value(handshakeKey{}).(*time.Timer); ok && handshakeUser(ctx) != nil {
			t.Stop()
		}
		return perms, nil
	}}
}

// channelContext is the context of one session channel: the connection's values
// and metadata, canceled when the channel closes or the connection ends.
type channelContext struct {
	ssh.Context
	done   context.Context
	cancel context.CancelFunc
	window atomic.Pointer[Window] // the last window change before shell or exec
}

func newChannelContext(ctx ssh.Context) *channelContext {
	c := &channelContext{Context: ctx}
	c.done, c.cancel = context.WithCancel(ctx)
	return c
}

func (c *channelContext) Deadline() (time.Time, bool) { return c.done.Deadline() }
func (c *channelContext) Done() <-chan struct{}       { return c.done.Done() }
func (c *channelContext) Err() error                  { return c.done.Err() }

// Value goes through the channel's cancel context, so a derived context finds
// it (context.Cause is per channel); other keys reach the connection's values,
// SetValue included.
func (c *channelContext) Value(k any) any { return c.done.Value(k) }

// earlyWindow is the last window change sent before shell or exec, nil when none.
func earlyWindow(ctx ssh.Context) *Window {
	if c, ok := ctx.(*channelContext); ok {
		return c.window.Load()
	}
	return nil
}

// sessionChannel serves a session channel with gliderlabs, minus window changes
// sent before shell or exec. gliderlabs buffers one window (the pty-req one) and
// blocks its request loop on the next until our handler drains them; the handler
// starts only on shell or exec, so an early window change would hang the channel
// forever, even after the connection closes. The session gets a context canceled
// when its channel closes, with or without a PTY.
func sessionChannel(srv *ssh.Server, conn *gossh.ServerConn, nc gossh.NewChannel, ctx ssh.Context) {
	cctx := newChannelContext(ctx)
	ssh.DefaultSessionHandler(srv, conn, earlyWindowFilter{nc, cctx}, cctx)
}

type earlyWindowFilter struct {
	gossh.NewChannel
	ctx *channelContext
}

// Accept relays the channel requests. Until a shell or exec passes, a window
// change is answered here as gliderlabs would (true only after a pty-req), and
// its size is kept as the initial window. The relay ends, and cancels the
// channel context, when the channel closes.
func (f earlyWindowFilter) Accept() (gossh.Channel, <-chan *gossh.Request, error) {
	ch, in, err := f.NewChannel.Accept()
	if err != nil {
		f.ctx.cancel()
		return ch, in, err
	}
	out := make(chan *gossh.Request)
	go func() {
		defer close(out)
		defer f.ctx.cancel() // x/crypto closes in on channel close and connection end
		started, sawPty := false, false
		for req := range in {
			switch {
			case started:
			case req.Type == "window-change":
				f.earlyWindowChange(req, sawPty, out)
				continue
			case req.Type == "pty-req":
				sawPty = true
			case req.Type == "shell" || req.Type == "exec":
				started = true
			}
			out <- req
		}
	}()
	return ch, out, nil
}

// earlyWindowChange keeps the size of a window change sent before shell or exec
// and answers it. Like gliderlabs, it reads cols and rows from the first 8
// bytes. Replies go in request order, so before replying it hands gliderlabs a
// sentinel: once gliderlabs takes it, every earlier request is
// answered (the sentinel wants no reply, so gliderlabs answers it with nothing).
func (f earlyWindowFilter) earlyWindowChange(req *gossh.Request, sawPty bool, out chan<- *gossh.Request) {
	var cols, rows uint32
	if p := req.Payload; len(p) >= 8 {
		cols, rows = binary.BigEndian.Uint32(p), binary.BigEndian.Uint32(p[4:])
	}
	ok := sawPty && cols > 0 && rows > 0
	if ok {
		f.ctx.window.Store(&Window{Cols: int(cols), Rows: int(rows)})
	}
	if req.WantReply {
		out <- &gossh.Request{Type: "werbot-sync"}
		_ = req.Reply(ok, nil)
	}
}
