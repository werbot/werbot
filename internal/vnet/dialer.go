//go:build linux || darwin

package vnet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/connectcli"
	"github.com/werbot/werbot/internal/platform/sshdgram"
	"github.com/werbot/werbot/internal/platform/version"
)

const handshakeTimeout = 30 * time.Second

// keepaliveEvery is the keepalive interval of a dialed connection; three
// unanswered in a row drop it. Tests shorten it.
var keepaliveEvery = 30 * time.Second

// gatewayDialer reaches resources through the gateway with one lazily dialed
// SSH connection per resource (user <login>+<resource>). A connection that
// drops, or misses its keepalives, is dialed again by the next call.
type gatewayDialer struct {
	login  string
	signer func(context.Context) (ssh.Signer, error)
	kbd    ssh.KeyboardInteractiveChallenge // one prompt at a time for all connections
	out    io.Writer                        // dial failures, refusals and channel messages for the user
	ctx    context.Context                  // detached from callers; Close cancels it
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu      sync.Mutex
	closed  bool
	clients map[clientKey]*client
}

// clientKey changes when the pool of a resource or its key changes.
type clientKey struct{ name, sshAddress, wsURL, hostKey string }

// client is a connection being dialed (ready open) or dialed (c or err set).
type client struct {
	ready chan struct{}
	c     *ssh.Client
	err   error
}

// newGatewayDialer: signer gives the user's certificate signer; open opens an
// MFA URL in a browser (nil prints it only, as under sudo).
func newGatewayDialer(login string, signer func(context.Context) (ssh.Signer, error), open func(string) error) *gatewayDialer {
	ctx, cancel := context.WithCancel(context.Background())
	return &gatewayDialer{
		login: login, signer: signer, out: os.Stderr, ctx: ctx, cancel: cancel,
		kbd:     connectcli.KeyboardInteractive(open, os.Stdin, os.Stderr),
		clients: map[clientKey]*client{},
	}
}

func (d *gatewayDialer) DialTCP(ctx context.Context, r Resource, port uint16) (io.ReadWriteCloser, error) {
	return d.open(ctx, r, "direct-tcpip", port)
}

func (d *gatewayDialer) DialDatagram(ctx context.Context, r Resource, port uint16) (io.ReadWriteCloser, error) {
	return d.open(ctx, r, sshdgram.ChannelType, port)
}

// open opens a channel of typ to the resource port. A connection that fails
// other than by a refusal of the gateway is dropped and dialed once more.
func (d *gatewayDialer) open(ctx context.Context, r Resource, typ string, port uint16) (io.ReadWriteCloser, error) {
	data := ssh.Marshal(sshdgram.OpenData{Host: r.Name, Port: uint32(port), OriginAddr: "127.0.0.1"})
	var err error
	for range 2 {
		var c *ssh.Client
		if c, err = d.client(ctx, r); err != nil {
			return nil, err
		}
		ch, reqs, oerr := c.OpenChannel(typ, data)
		if oerr == nil {
			go ssh.DiscardRequests(reqs)
			go func() { _, _ = io.Copy(d.out, ch.Stderr()) }()
			return ch, nil
		}
		if oe, refused := errors.AsType[*ssh.OpenChannelError](oerr); refused {
			_, _ = fmt.Fprintf(d.out, "%s: %s\n", r.Name, oe.Message)
			return nil, oerr
		}
		err = oerr
		_ = c.Close()
		d.forget(keyOf(r), c)
	}
	return nil, err
}

func keyOf(r Resource) clientKey { return clientKey{r.Name, r.SSHAddress, r.WSURL, r.HostKey} }

// client is the connection of r, dialed if needed. The dial is detached from
// ctx: a caller that gives up leaves it to finish for the next one.
func (d *gatewayDialer) client(ctx context.Context, r Resource) (*ssh.Client, error) {
	k := keyOf(r)
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil, net.ErrClosed
	}
	cl := d.clients[k]
	if cl == nil {
		cl = &client{ready: make(chan struct{})}
		d.clients[k] = cl
		d.wg.Go(func() { d.connect(k, r, cl) })
	}
	d.mu.Unlock()
	select {
	case <-cl.ready:
		return cl.c, cl.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (d *gatewayDialer) connect(k clientKey, r Resource, cl *client) {
	cl.c, cl.err = d.dial(r)
	close(cl.ready)
	if cl.err != nil && d.ctx.Err() == nil {
		_, _ = fmt.Fprintf(d.out, "%s: %v\n", r.Name, cl.err)
	}
	if cl.err == nil {
		kctx, kstop := context.WithCancel(d.ctx)
		go connectcli.Keepalive(kctx, cl.c, keepaliveEvery)
		stop := context.AfterFunc(d.ctx, func() { _ = cl.c.Close() })
		_ = cl.c.Wait()
		stop()
		kstop()
	}
	d.forget(k, cl.c)
}

// forget drops the dialed entry of k if it holds c (nil: a failed dial).
func (d *gatewayDialer) forget(k clientKey, c *ssh.Client) {
	d.mu.Lock()
	defer d.mu.Unlock()
	cl := d.clients[k]
	if cl == nil {
		return
	}
	select {
	case <-cl.ready:
		if cl.c == c {
			delete(d.clients, k)
		}
	default: // still dialing
	}
}

// dial connects over SSH or WebSocket and checks the pool host key.
func (d *gatewayDialer) dial(r Resource) (*ssh.Client, error) {
	hk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(r.HostKey))
	if err != nil {
		return nil, fmt.Errorf("%s: gateway host key: %w", r.Name, err)
	}
	conf := &ssh.ClientConfig{
		User: d.login + "+" + r.Name,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
				s, err := d.signer(d.ctx)
				return []ssh.Signer{s}, err
			}),
			ssh.KeyboardInteractive(d.kbd),
		},
		HostKeyCallback: ssh.FixedHostKey(hk),
		ClientVersion:   "SSH-2.0-Werbot_vnet_" + strings.ReplaceAll(version.Version, "-", "_"),
	}
	ep := connectcli.Endpoint{SSHAddress: r.SSHAddress, WSURL: r.WSURL}
	conn, err := connectcli.Dial(d.ctx, ep)
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(d.ctx, func() { _ = conn.Close() })
	defer stop()
	_ = conn.SetDeadline(time.Now().Add(handshakeTimeout))
	c, chans, reqs, err := ssh.NewClientConn(conn, connectcli.HostKeyAddr(ep), conf)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return ssh.NewClient(c, chans, reqs), nil
}

// Close closes every connection and stops dials in progress.
func (d *gatewayDialer) Close() error {
	d.mu.Lock()
	d.closed = true
	d.mu.Unlock()
	d.cancel()
	d.wg.Wait()
	return nil
}
