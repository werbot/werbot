// Package sshtest is a test helper: an in-process SSH target on gliderlabs/ssh.
// It is for _test.go files only.
package sshtest

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

// Target is a running SSH target.
type Target struct {
	Addr, EchoAddr, Password string // EchoAddr is a TCP echo reachable through direct-tcpip
	HostKey                  gossh.PublicKey
	Events                   chan string           // "pty:<term> <cols>x<rows>", "win:<cols>x<rows>", "signal:<name>", "closed"
	exec                     func(ssh.Session) int // runs a command instead of run; set by an Option
	noRemoteForward          bool                  // refuses tcpip-forward; set by NoRemoteForward
}

// Option changes how a target runs commands or forwards.
type Option func(*Target)

// NoRemoteForward makes the target refuse tcpip-forward.
func NoRemoteForward(tg *Target) { tg.noRemoteForward = true }

// StartTarget starts a target that accepts password (if not empty) and the
// authorized key (if not nil); with neither it needs no authentication.
// It serves direct-tcpip and tcpip-forward (unless NoRemoteForward).
// The shell echoes input line by line; exec runs "echo X" ("X\n"), "exit N"
// (exit code N), "cat" (stdin to stdout), "hostname" ("target\n"), "uname"
// ("Linux\n") and "sleep N" (N seconds, or until the client leaves), unless
// an option changes it. Everything stops at the test cleanup.
func StartTarget(t testing.TB, password string, authorized gossh.PublicKey, opts ...Option) *Target {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("sshtest: host key: %v", err)
	}
	signer, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("sshtest: host signer: %v", err)
	}
	tg := &Target{Password: password, HostKey: signer.PublicKey(), Events: make(chan string, 64), exec: run}
	for _, o := range opts {
		o(tg)
	}
	srv := &ssh.Server{
		Handler:                     tg.handle,
		HostSigners:                 []ssh.Signer{signer},
		LocalPortForwardingCallback: func(ssh.Context, string, uint32) bool { return true },
		ChannelHandlers:             map[string]ssh.ChannelHandler{"session": ssh.DefaultSessionHandler, "direct-tcpip": ssh.DirectTCPIPHandler},
	}
	fwd := &ssh.ForwardedTCPHandler{}
	srv.RequestHandlers = map[string]ssh.RequestHandler{"tcpip-forward": fwd.HandleSSHRequest, "cancel-tcpip-forward": fwd.HandleSSHRequest}
	srv.ReversePortForwardingCallback = func(ssh.Context, string, uint32) bool { return !tg.noRemoteForward }
	if password != "" {
		srv.PasswordHandler = func(_ ssh.Context, p string) bool {
			return subtle.ConstantTimeCompare([]byte(p), []byte(password)) == 1
		}
	}
	if authorized != nil {
		srv.PublicKeyHandler = func(_ ssh.Context, k ssh.PublicKey) bool { return ssh.KeysEqual(k, authorized) }
	}

	ln := listen(t)
	tg.Addr = ln.Addr().String()
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	echo := listen(t)
	tg.EchoAddr = echo.Addr().String()
	go serveEcho(echo)
	t.Cleanup(func() { _ = echo.Close() })
	return tg
}

func listen(t testing.TB) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("sshtest: listen: %v", err)
	}
	return ln
}

func serveEcho(ln net.Listener) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go func() {
			defer c.Close()
			_, _ = io.Copy(c, c)
		}()
	}
}

// event records e; it never blocks the target, a full Events drops e.
func (tg *Target) event(e string) {
	select {
	case tg.Events <- e:
	default:
	}
}

func (tg *Target) handle(s ssh.Session) {
	defer tg.event("closed")
	if pty, winch, ok := s.Pty(); ok {
		tg.event(fmt.Sprintf("pty:%s %dx%d", pty.Term, pty.Window.Width, pty.Window.Height))
		go tg.windows(winch)
	}
	if s.RawCommand() == "" {
		shell(s)
		return
	}
	_ = s.Exit(tg.exec(s))
}

// windows reports size changes. gliderlabs puts the pty-req size into the
// channel first (session.go:337-338); it is the "pty:" event, so it is dropped.
func (tg *Target) windows(winch <-chan ssh.Window) {
	<-winch
	for w := range winch {
		tg.event(fmt.Sprintf("win:%dx%d", w.Width, w.Height))
	}
}

// shell echoes every input line (ended by CR or LF) until EOF.
func shell(s ssh.Session) {
	sc := bufio.NewScanner(s)
	sc.Split(scanLines)
	for sc.Scan() {
		if _, err := fmt.Fprintf(s, "%s\n", sc.Bytes()); err != nil {
			return
		}
	}
}

func scanLines(data []byte, atEOF bool) (int, []byte, error) {
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// run executes a command and returns its exit code.
func run(s ssh.Session) int {
	args := s.Command()
	switch {
	case args[0] == "echo":
		_, _ = fmt.Fprintln(s, strings.Join(args[1:], " "))
	case args[0] == "exit" && len(args) == 2:
		code, err := strconv.Atoi(args[1])
		if err != nil {
			return 2
		}
		return code
	case args[0] == "cat":
		_, _ = io.Copy(s, s)
	case args[0] == "hostname":
		_, _ = io.WriteString(s, "target\n")
	default:
		return other(s, args)
	}
	return 0
}

// other runs "uname" and "sleep N"; anything else is not found.
func other(s ssh.Session, args []string) int {
	switch {
	case args[0] == "uname":
		_, _ = io.WriteString(s, "Linux\n")
		return 0
	case args[0] == "sleep" && len(args) == 2:
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return 2
		}
		select {
		case <-time.After(time.Duration(n) * time.Second):
			return 0
		case <-s.Context().Done():
			return 1
		}
	}
	_, _ = fmt.Fprintf(s.Stderr(), "%s: command not found\n", args[0])
	return 127
}
