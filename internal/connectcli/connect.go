package connectcli

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"slices"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/platform/sshdgram"
	"github.com/werbot/werbot/internal/platform/version"
)

// Run opens a local port for each spec in cfg and serves it until ctx ends,
// then returns nil. A failure to open a port, a refused resource or a changed
// gateway host key returns an error; gateway texts go to stderr as they are.
func Run(ctx context.Context, cfg Config, stdout, stderr io.Writer) error {
	stderr = &lockedWriter{w: stderr}
	ctx, cancel := context.WithCancel(ctx)
	auth, closeAuth, err := cfg.auth(ctx)
	if err != nil {
		cancel()
		return err
	}
	defer closeAuth()
	var wg sync.WaitGroup
	defer func() { cancel(); wg.Wait() }() // every port closed, every goroutine done
	fail := make(chan error, len(cfg.Specs))
	ep := Endpoint{SSHAddress: cfg.Gateway, WSURL: cfg.WSURL}
	for _, sp := range cfg.Specs {
		conf := &ssh.ClientConfig{
			User: cfg.Login + "+" + sp.Resource, Auth: []ssh.AuthMethod{auth, cfg.mfaAuth(stderr)},
			HostKeyCallback: hostKeyCallback(cfg.KnownHosts), ClientVersion: "SSH-2.0-Werbot_connect_" + strings.ReplaceAll(version.Version, "-", "_"),
		}
		b, err := open(ctx, ep, conf, sp, stdout, stderr)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		wg.Go(func() {
			if err := b.serve(ctx); err != nil {
				fail <- err
			}
		})
	}
	select {
	case <-ctx.Done():
		return nil
	case err := <-fail:
		return err
	}
}

// binding is one served spec: its link and local port.
type binding struct {
	link   *link
	target uint32
	ln     net.Listener   // tcp
	pc     net.PacketConn // udp
	stderr io.Writer
}

// open connects as <login>+<resource>, asks the gateway for the resource,
// opens the local port and prints where it listens.
func open(ctx context.Context, ep Endpoint, conf *ssh.ClientConfig, sp Spec, stdout, stderr io.Writer) (*binding, error) {
	c, err := DialSSH(ctx, ep, conf)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", sp.Resource, err)
	}
	b, err := setup(ctx, c, sp, stdout, stderr)
	if err != nil {
		_ = c.Close()
		return nil, err
	}
	b.link = newLink(c, ep, conf, sp.Resource, stderr)
	return b, nil
}

// RequestResource asks the gateway behind c for the resource its login
// names. A refusal is the gateway's text ("Access denied" when it gives none)
// with a zero info and a nil error.
func RequestResource(c *ssh.Client) (info sshdgram.ResourceInfo, refusal string, err error) {
	ok, payload, err := c.SendRequest(sshdgram.ResourceRequest, true, nil)
	if err != nil {
		return info, "", err
	}
	if !ok {
		return info, cmp.Or(string(payload), "Access denied"), nil // the gateway may refuse without a reason
	}
	if err := json.Unmarshal(payload, &info); err != nil {
		return info, "", fmt.Errorf("resource info: %w", err)
	}
	return info, "", nil
}

// setup reads the resource info through c and opens the local port.
func setup(ctx context.Context, c *ssh.Client, sp Spec, stdout, stderr io.Writer) (*binding, error) {
	stop := context.AfterFunc(ctx, func() { _ = c.Close() })
	defer stop()
	info, refusal, err := RequestResource(c)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", sp.Resource, err)
	}
	if refusal != "" {
		_, _ = fmt.Fprintln(stderr, refusal)
		return nil, fmt.Errorf("%s: refused by the gateway", sp.Resource)
	}
	udp := slices.Contains(info.Kinds, "datagram")
	if !udp && !slices.Contains(info.Kinds, "stream") {
		return nil, fmt.Errorf("Resource %s does not support port access", sp.Resource) //nolint:staticcheck // the gateway's capitalized text
	}
	target, err := targetPort(sp, info.Ports)
	if err != nil {
		return nil, err
	}
	local := sp.LocalPort
	if local == 0 {
		local = DefaultLocalPort(target)
	}
	ln, pc, err := listen(udp, local, sp.Resource)
	if err != nil {
		return nil, err
	}
	proto := "tcp"
	if udp {
		proto = "udp"
	}
	_, _ = fmt.Fprintf(stdout, "%s → 127.0.0.1:%d (%s)\n", sp.Resource, local, proto)
	return &binding{target: uint32(target), ln: ln, pc: pc, stderr: stderr}, nil
}

// serve runs the link and the local port until ctx ends or the link fails.
func (b *binding) serve(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	if b.ln != nil {
		wg.Go(func() { b.serveTCP(ctx) })
	} else {
		wg.Go(func() { newUDPRelay(b).serve(ctx) })
	}
	err := b.link.run(ctx)
	cancel()
	if b.ln != nil {
		_ = b.ln.Close()
	} else {
		_ = b.pc.Close()
	}
	wg.Wait()
	return err
}

// OpenData is the channel open data of direct-tcpip and datagram@werbot.com:
// the resource name and the target port, from the local address when there is one.
func OpenData(resource string, port uint32, from net.Addr) []byte {
	d := sshdgram.OpenData{Host: resource, Port: port, OriginAddr: "127.0.0.1"}
	if a, ok := from.(interface{ AddrPort() netip.AddrPort }); ok {
		d.OriginPort = uint32(a.AddrPort().Port())
	}
	return ssh.Marshal(d)
}

// openChannel opens a channel of typ through the link.
func (b *binding) openChannel(ctx context.Context, typ string, from net.Addr) (ssh.Channel, <-chan *ssh.Request, error) {
	c, err := b.link.get(ctx)
	if err != nil {
		return nil, nil, err
	}
	return c.OpenChannel(typ, OpenData(b.link.resource, b.target, from))
}

// openText is what the user reads about a failed channel open: a gateway
// refusal as it is, any other error with the resource name.
func (b *binding) openText(err error) string {
	if oe, ok := errors.AsType[*ssh.OpenChannelError](err); ok {
		return oe.Message
	}
	return b.link.resource + ": " + err.Error()
}

// lockedWriter makes one writer safe for the goroutines of all bindings.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}
