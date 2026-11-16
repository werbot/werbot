package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/gliderlabs/ssh"
	"github.com/pires/go-proxyproto"
	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/platform/version"
)

// sshFront is the SSH entry: public key login, the resource menu, shell, exec and direct-tcpip.
type sshFront struct {
	cfg              Config
	hostKey          gossh.Signer
	log              *slog.Logger
	handshakeTimeout time.Duration // closes a connection that has not authenticated by then
}

var _ Front = (*sshFront)(nil)

func newSSHFront(cfg Config, hostKey gossh.Signer, logger *slog.Logger) *sshFront {
	return &sshFront{cfg: cfg, hostKey: hostKey, log: logger, handshakeTimeout: 30 * time.Second}
}

func (*sshFront) Name() string { return "ssh" }

// Serve accepts SSH connections on ln until ctx ends, then closes ln. Open
// connections keep running while the pipeline drains and are closed once it stops.
func (f *sshFront) Serve(ctx context.Context, ln net.Listener, p *Pipeline) error {
	h := &sshHandler{p: p, log: f.log}
	srv := &ssh.Server{
		Version:          "Werbot_" + strings.ReplaceAll(version.Version, "-", "_"),
		IdleTimeout:      f.cfg.IdleTimeout,
		PublicKeyHandler: h.auth,
		Handler:          h.session,
		ChannelHandlers:  map[string]ssh.ChannelHandler{"session": sessionChannel, "direct-tcpip": h.forward},
		ConnCallback: func(ctx ssh.Context, conn net.Conn) net.Conn {
			return newHandshakeConn(ctx, conn, f.handshakeTimeout)
		},
		ServerConfigCallback: authenticated,
	}
	srv.AddHostKey(f.hostKey)
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()
	select {
	case err := <-served: // the listener failed
		_ = srv.Close()
		return fmt.Errorf("ssh front: %w", err)
	case <-ctx.Done():
	}
	_ = ln.Close()
	<-served
	<-p.stop.Done()
	_ = srv.Close() // connections left in the menu or without a session
	return nil
}

// wrapListener makes ln require a PROXY protocol header on every connection when proxyProtocol is set.
func wrapListener(ln net.Listener, proxyProtocol bool) net.Listener {
	if !proxyProtocol {
		return ln
	}
	return &proxyproto.Listener{Listener: ln, ReadHeaderTimeout: 5 * time.Second} // default policy: REQUIRE
}

// userKey holds the *User accepted at the handshake; nil while the control plane was unreachable.
type userKey struct{}

type sshHandler struct {
	p   *Pipeline
	log *slog.Logger
}

// auth checks the key and the login. x/crypto also calls it for keys the client
// only offers, and the last call is for the key that signs, so the user is
// overwritten on every call: one found for an offered key never carries over
// (CVE-2024-45337). An unreachable control plane accepts the key without a user;
// the signature is still checked and every channel gets the not ready text.
func (h *sshHandler) auth(ctx ssh.Context, key ssh.PublicKey) bool {
	ctx.SetValue(userKey{}, (*User)(nil))
	login, _, err := ParseLogin(ctx.User())
	if err != nil {
		return false
	}
	actx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ip := clientIP(ctx.RemoteAddr())
	u, err := h.p.ctl.AuthenticateUser(actx, gossh.FingerprintSHA256(key), ip)
	switch {
	case unreachable(err):
		return true
	case err != nil:
		lvl := slog.LevelWarn
		if c := connect.CodeOf(err); c == connect.CodeNotFound || c == connect.CodePermissionDenied || c == connect.CodeUnauthenticated {
			lvl = slog.LevelInfo // an unknown or refused key
		}
		h.log.Log(ctx, lvl, "ssh: key refused", "login", login, "client_ip", ip, "error", err)
		return false
	case u.Login != login:
		h.log.Info("ssh: key belongs to another login", "login", login, "client_ip", ip)
		return false
	}
	ctx.SetValue(userKey{}, &u)
	return true
}

// handshakeUser is the user accepted at the handshake; nil means not ready.
func handshakeUser(ctx ssh.Context) *User {
	u, _ := ctx.Value(userKey{}).(*User)
	return u
}

// forward serves direct-tcpip: the channel is accepted only after Open succeeded,
// so an unreachable target closes an accepted channel and a refusal is Prohibited.
func (h *sshHandler) forward(_ *ssh.Server, conn *gossh.ServerConn, nc gossh.NewChannel, ctx ssh.Context) {
	var d struct {
		DestAddr   string
		DestPort   uint32
		OriginAddr string
		OriginPort uint32
	}
	if err := gossh.Unmarshal(nc.ExtraData(), &d); err != nil {
		_ = nc.Reject(gossh.ConnectionFailed, "invalid direct-tcpip request")
		return
	}
	_, resource, _ := ParseLogin(conn.User()) // checked at the handshake
	switch {
	case handshakeUser(ctx) == nil:
		_ = nc.Reject(gossh.Prohibited, openText(h.p.reject(ErrNotReady), resource, KindStream, nil))
		return
	case resource == "":
		_ = nc.Reject(gossh.Prohibited, usageText)
		return
	}
	target := net.JoinHostPort(d.DestAddr, strconv.FormatUint(uint64(d.DestPort), 10))
	sess, err := h.p.Open(ctx, h.request(ctx, resource, KindStream, Window{}, target))
	if err != nil {
		_ = nc.Reject(gossh.Prohibited, openText(err, resource, KindStream, h.p.ctl.Policy()))
		return
	}
	c, ok := LookupConnector(sess.Resource.Type)
	if !ok { // Open found it; registrations are never removed
		h.p.abort(sess)
		_ = nc.Reject(gossh.Prohibited, openText(ErrUnsupportedType, resource, KindStream, h.p.ctl.Policy()))
		return
	}
	ch, reqs, err := nc.Accept()
	if err != nil {
		h.p.abort(sess)
		return
	}
	defer ch.Close() //nolint:errcheck // the client may be gone
	go gossh.DiscardRequests(reqs)
	sess.Stream = ch
	_ = h.p.Run(ctx, sess, c) // the pipeline logs connector errors
}

// request is the OpenRequest of the key that signed the handshake.
func (h *sshHandler) request(ctx ssh.Context, resource string, kind Kind, w Window, target string) OpenRequest {
	key, _ := ctx.Value(ssh.ContextKeyPublicKey).(ssh.PublicKey) // set by the last accepted key: the one that signed
	fp := ""
	if key != nil {
		fp = gossh.FingerprintSHA256(key)
	}
	return OpenRequest{Front: "ssh", Kind: kind, Identity: Identity{Method: "ssh-key", Value: fp},
		Resource: resource, Target: target, ClientIP: clientIP(ctx.RemoteAddr()), Window: w}
}

// clientIP is the client address; with the PROXY protocol, the one from the header.
func clientIP(a net.Addr) netip.Addr {
	if t, ok := a.(*net.TCPAddr); ok {
		return t.AddrPort().Addr().Unmap()
	}
	return netip.Addr{}
}

const usageText = "Use ssh <login>+<resource>@<host> to run commands or forward ports"

// openText is the user text for an Open error; denied and unknown resources read the same.
func openText(err error, resource string, kind Kind, pol *Policy) string {
	switch {
	case errors.Is(err, ErrNotReady):
		return "Gateway is not ready, try again later"
	case errors.Is(err, ErrBusy):
		return "Gateway is busy, try again later"
	case errors.Is(err, ErrUnsupportedType):
		r, _ := pol.Resource(resource)
		return fmt.Sprintf("Resource type %q is not supported by this gateway", r.Type)
	case errors.Is(err, ErrUnsupportedKind):
		return fmt.Sprintf("Resource %s does not support %s sessions", resource, kind)
	}
	return "Access denied"
}

// runText is the user text for a connector error.
func runText(err error, resource string) string {
	if errors.Is(err, ErrHostKeyMismatch) {
		return "SSH host key mismatch for " + resource
	}
	return "Connection to " + resource + " failed"
}

// say writes msg as one line with CRLF line ends, as a terminal needs them.
func say(w io.Writer, msg string) {
	_, _ = io.WriteString(w, strings.ReplaceAll(msg, "\n", "\r\n")+"\r\n")
}
