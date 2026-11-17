// Package telnetconnector is the gateway connector of the "telnet" resource
// type: a terminal through the SSH front, or a stream for a local telnet client
// (the gateway is the telnet server to that client), both with an automatic
// login when the resource has a login and a password.
//
// The gateway → target leg is plain text, as telnet is: run the gateway next to
// the equipment, in an isolated segment.
package telnetconnector

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	telnetv1 "github.com/werbot/werbot/gen/werbot/connector/telnet/v1"
	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/gateway"
)

func init() { gateway.RegisterConnector(Connector{}) }

const (
	loginTimeout  = 10 * time.Second // the whole automatic login
	negotiateWait = time.Second      // a stream client's TERM and window before the dial
	defaultTerm   = "xterm"
	defaultPort   = 23
)

// Connector serves "telnet" sessions.
type Connector struct{}

// Type is "telnet".
func (Connector) Type() string { return "telnet" }

// Kinds is terminal and stream.
func (Connector) Kinds() []gateway.Kind {
	return []gateway.Kind{gateway.KindTerminal, gateway.KindStream}
}

// DialsTarget marks the connector as dialing through Session.Dial.
func (Connector) DialsTarget() {}

// Serve connects to the target and serves s until either side ends; a cancel
// of ctx closes the target connection (and the stream) and is not an error. A
// relay goroutine panic stops the session and re-panics here for the pipeline.
func (Connector) Serve(ctx context.Context, s *gateway.Session) error {
	sctx, stop := context.WithCancel(ctx)
	defer stop()
	g := gateway.NewGuard(stop, s.Secret)
	if s.Kind == gateway.KindStream {
		defer s.Stream.Close()
		defer context.AfterFunc(sctx, func() { _ = s.Stream.Close() })()
	}
	err := serve(sctx, s, g)
	stop() // ends the guarded goroutines
	if werr := g.Wait(); err == nil {
		err = werr
	}
	if ctx.Err() != nil {
		return nil
	}
	return err
}

func serve(ctx context.Context, s *gateway.Session, g *gateway.Guard) error {
	var cfg telnetv1.Config
	if err := protojson.Unmarshal(s.Resource.Config, &cfg); err != nil {
		return fmt.Errorf("telnet: resource config: %w", err)
	}
	switch s.Kind {
	case gateway.KindTerminal:
		return terminal(ctx, s, g, &cfg)
	case gateway.KindStream:
		return stream(ctx, s, g, &cfg)
	default:
		return fmt.Errorf("telnet: unsupported session kind %q", s.Kind)
	}
}

// terminal bridges the SSH terminal to the target: TERM and the window from
// pty-req, window changes as NAWS.
func terminal(ctx context.Context, s *gateway.Session, g *gateway.Guard, cfg *telnetv1.Config) error {
	t := s.Terminal
	if t.Command != "" {
		return errors.New("telnet: commands are not supported")
	}
	tc, err := dial(ctx, s, cfg, cmp.Or(t.Term, defaultTerm), t.Window)
	if err != nil {
		return err
	}
	defer tc.Close()
	defer context.AfterFunc(ctx, func() { _ = tc.Close() })()
	pw := password(s, cfg.GetLogin())
	out := newRedactor(io.MultiWriter(recordFunc(s.Recording.Output), t), pw)
	if err := autologin(ctx, tc, cfg.GetLogin(), pw, out); err != nil {
		return err
	}
	return relay(ctx, g, tc, t, out, s.Recording, t.Resize, t.Window)
}

// dial connects to the target through Session.Dial and stores the window to
// send once the target asks for NAWS.
func dial(ctx context.Context, s *gateway.Session, cfg *telnetv1.Config, term string, w gateway.Window) (*Conn, error) {
	port := cmp.Or(cfg.GetPort(), defaultPort)
	conn, err := s.Dial(ctx, "tcp", net.JoinHostPort(cfg.GetHost(), strconv.FormatUint(uint64(port), 10)))
	if err != nil {
		return nil, fmt.Errorf("telnet: dial target: %w", err)
	}
	tc := NewClient(conn, term)
	if w.Cols > 0 && w.Rows > 0 {
		_ = tc.SetWindowSize(w.Cols, w.Rows) // only stored: NAWS is not agreed yet
	}
	return tc, nil
}

// password is the password the gateway logs in with: set only when the
// resource has a login and the credential is a password.
func password(s *gateway.Session, login string) string {
	if login == "" || connector.CredentialKind(s.Secret.Kind) != connector.CredentialPassword {
		return ""
	}
	return string(s.Secret.Value)
}

// autologin logs in when pw is set and writes the output after the password
// prompt to out (the recorded, redacted user output). The login and password
// lines go to the target only, so they are not recorded. A refused or
// timed-out login is gateway.ErrLoginFailed.
func autologin(ctx context.Context, tc *Conn, login, pw string, out io.Writer) error {
	if pw == "" {
		return nil
	}
	lctx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()
	text, err := Login(lctx, tc, login, pw)
	if errors.Is(err, ErrLoginFailed) || errors.Is(err, ErrLoginTimeout) {
		return fmt.Errorf("telnet: %w: %w", gateway.ErrLoginFailed, err)
	}
	if err != nil {
		return fmt.Errorf("telnet: login: %w", err)
	}
	_, err = out.Write(text)
	return err
}
