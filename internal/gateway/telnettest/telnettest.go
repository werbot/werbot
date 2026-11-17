// Package telnettest is a test helper: an in-process telnet target with a
// login prompt and an echo shell. It is for _test.go files only.
package telnettest

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"

	telnetv1 "github.com/werbot/werbot/gen/werbot/connector/telnet/v1"
	_ "github.com/werbot/werbot/internal/connector/telnet" // the server type "telnet"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/connectortest"
	"github.com/werbot/werbot/internal/gateway/telnetconnector"
)

// Login is the account the target accepts.
const Login = "werbot"

// negotiateWait bounds the wait for the client's NAWS and TTYPE before the prompt.
const negotiateWait = 2 * time.Second

// Server is a running telnet target. It echoes every byte it reads, the login
// and password lines too (as a careless device does), and after a login with
// Login and Password it is an echo shell.
type Server struct {
	Addr     string
	Password string
	Accepts  func() int // connections accepted so far
	// Events: "prompt <cols>x<rows> <term>" when the first login prompt is
	// printed, "login <name>" on each attempt, "win <cols>x<rows>" for NAWS
	// after the prompt. Sends never block; extra events are dropped.
	Events chan string

	mu    sync.Mutex
	shell bytes.Buffer // what the shell read after the login
	wg    sync.WaitGroup
}

// Target is a connectortest target: resource "router" of type telnet with the
// login Login, its password a unique marker, and the stream address of Start.
func Target(t testing.TB) connectortest.Target {
	t.Helper()
	srv := Start(t, "werbot-telnet-secret-"+rand.Text())
	return connectortest.Target{
		Resource: srv.Resource(t, Login),
		Secret:   gateway.Secret{Kind: "password", Value: []byte(srv.Password)},
		Stream:   srv.Addr,
		Accepts:  srv.Accepts,
	}
}

// Start starts a target that accepts Login with password; it stops at the test cleanup.
func Start(t testing.TB, password string) *Server {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("telnettest: listen: %v", err)
	}
	l, accepts := connectortest.CountAccepts(l)
	s := &Server{Addr: l.Addr().String(), Password: password, Accepts: accepts, Events: make(chan string, 64)}
	var conns sync.Map
	s.wg.Go(func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			conns.Store(c, struct{}{})
			s.wg.Go(func() {
				defer conns.Delete(c)
				defer c.Close()
				s.handle(c)
			})
		}
	})
	t.Cleanup(func() {
		_ = l.Close()
		conns.Range(func(c, _ any) bool { _ = c.(net.Conn).Close(); return true })
		s.wg.Wait()
	})
	return s
}

// Resource is resource "router" of type telnet at the server with the given login.
func (s *Server) Resource(t testing.TB, login string) gateway.Resource {
	t.Helper()
	return Resource(t, s.Addr, login)
}

// Resource is resource "router" of type telnet at addr (host:port) with the given login.
func Resource(t testing.TB, addr, login string) gateway.Resource {
	t.Helper()
	host, port, _ := net.SplitHostPort(addr)
	p, _ := strconv.ParseUint(port, 10, 16)
	cfg, err := protojson.Marshal(&telnetv1.Config{Host: host, Port: uint32(p), Login: login})
	if err != nil {
		t.Fatalf("telnettest: config: %v", err)
	}
	return gateway.Resource{ID: uuid.New(), Name: "router", Type: "telnet", Config: cfg}
}

// Shell returns what the shell read after the login.
func (s *Server) Shell() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.shell.String()
}

func (s *Server) event(format string, args ...any) {
	select {
	case s.Events <- fmt.Sprintf(format, args...):
	default:
	}
}

// handle serves one connection: negotiation, up to three login attempts, then
// the echo shell until the client goes.
func (s *Server) handle(nc net.Conn) {
	c, err := telnetconnector.NewServer(nc)
	if err != nil {
		return
	}
	prompted := make(chan struct{})
	c.OnResize = func(cols, rows int) {
		select {
		case <-prompted:
			s.event("win %dx%d", cols, rows)
		default:
		}
	}
	chunks, done := make(chan []byte, 16), make(chan struct{})
	defer close(done)
	s.wg.Go(func() {
		defer close(chunks)
		buf := make([]byte, 4096)
		for {
			n, err := c.Read(buf)
			if n > 0 {
				_, _ = c.Write(buf[:n]) // echo
				select {
				case chunks <- bytes.Clone(buf[:n]):
				case <-done:
					return
				}
			}
			if err != nil {
				return
			}
		}
	})

	select {
	case <-c.Negotiated():
	case <-time.After(negotiateWait):
	}
	cols, rows := c.Size()
	s.event("prompt %dx%d %s", cols, rows, c.TermType())
	close(prompted)

	in := &lineReader{chunks: chunks}
	for range 3 {
		_, _ = c.Write([]byte("login: "))
		name, ok := in.line()
		if !ok {
			return
		}
		s.event("login %s", name)
		_, _ = c.Write([]byte("Password: "))
		pass, ok := in.line()
		if !ok {
			return
		}
		if name == Login && pass == s.Password {
			_, _ = c.Write([]byte("Welcome\r\n$ "))
			s.record(bytes.TrimLeft(in.rest, "\n\x00"))
			for p := range chunks {
				s.record(p)
			}
			return
		}
		_, _ = c.Write([]byte("Login incorrect\r\n"))
	}
}

func (s *Server) record(p []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shell.Write(p)
}

// lineReader splits the chunks into lines ended by CR or LF; the LF of a CR LF
// pair and NULs are skipped.
type lineReader struct {
	chunks <-chan []byte
	rest   []byte
}

func (r *lineReader) line() (string, bool) {
	for {
		r.rest = bytes.TrimLeft(r.rest, "\n\x00")
		if i := bytes.IndexAny(r.rest, "\r\n"); i >= 0 {
			line := string(r.rest[:i])
			r.rest = r.rest[i+1:]
			return line, true
		}
		p, ok := <-r.chunks
		if !ok {
			return "", false
		}
		r.rest = append(r.rest, p...)
	}
}
