// Package sshconnector is the gateway connector of the "ssh" resource type: a
// terminal or an exec in a target session, or a stream through the target
// (direct-tcpip). The target host key is pinned through the server on first use.
package sshconnector

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/encoding/protojson"

	sshv1 "github.com/werbot/werbot/gen/werbot/connector/ssh/v1"
	"github.com/werbot/werbot/internal/gateway"
)

func init() { gateway.RegisterConnector(Connector{DialTimeout: 10 * time.Second}) }

// Connector serves "ssh" sessions; DialTimeout bounds the dial and the handshake.
type Connector struct{ DialTimeout time.Duration }

// Type is "ssh".
func (Connector) Type() string { return "ssh" }

// Kinds is terminal, exec and stream.
func (Connector) Kinds() []gateway.Kind {
	return []gateway.Kind{gateway.KindTerminal, gateway.KindExec, gateway.KindStream}
}

// Serve connects to the target and serves s until it ends; a cancel of ctx
// closes the target connection (and the stream) and is not an error. A relay
// panic closes the target and re-panics here, cancel or not, for the pipeline.
func (c Connector) Serve(ctx context.Context, s *gateway.Session) error {
	if s.Kind == gateway.KindStream {
		defer s.Stream.Close()
		defer context.AfterFunc(ctx, func() { _ = s.Stream.Close() })()
	}
	err := c.serve(ctx, s)
	if ctx.Err() != nil {
		return nil
	}
	return err
}

func (c Connector) serve(ctx context.Context, s *gateway.Session) error {
	var cfg sshv1.Config
	if err := protojson.Unmarshal(s.Resource.Config, &cfg); err != nil {
		return fmt.Errorf("ssh: resource config: %w", err)
	}
	port := cfg.GetPort()
	if port == 0 {
		port = 22
	}
	addr := net.JoinHostPort(cfg.GetHost(), strconv.FormatUint(uint64(port), 10))
	ccfg, err := clientConfig(cfg.GetLogin(), s.Secret, s.PinKey, c.DialTimeout)
	if err != nil {
		return err
	}

	conn, err := (&net.Dialer{Timeout: c.DialTimeout}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("ssh: dial target: %w", err)
	}
	defer conn.Close()
	defer context.AfterFunc(ctx, func() { _ = conn.Close() })()
	client, err := c.handshake(conn, addr, ccfg)
	if err != nil {
		return err
	}
	defer client.Close()

	switch s.Kind {
	case gateway.KindTerminal:
		return terminalCommand(s.Terminal).run(ctx, client, s.Recording, s.Secret)
	case gateway.KindExec:
		return execCommand(s.Exec).run(ctx, client, s.Recording, s.Secret)
	case gateway.KindStream:
		target, err := client.Dial("tcp", dialTarget(s.Resource.Name, s.Target)) // direct-tcpip at the target
		if err != nil {
			return fmt.Errorf("ssh: open %s through the target: %w", s.Target, err)
		}
		return pipe(s.Stream, target, s.Secret)
	default:
		return fmt.Errorf("ssh: unsupported session kind %q", s.Kind)
	}
}

// dialTarget is the address the target dials for a stream: a host equal to the
// resource name (as `werbot connect` sends it) is the target itself, localhost.
func dialTarget(resourceName, target string) string {
	host, port, err := net.SplitHostPort(target)
	if err != nil || !strings.EqualFold(host, resourceName) {
		return target
	}
	return net.JoinHostPort("localhost", port)
}

// handshake runs the SSH handshake on conn within DialTimeout. A host key the
// server refuses fails it with an error wrapping gateway.ErrHostKeyMismatch.
func (c Connector) handshake(conn net.Conn, addr string, cfg *gossh.ClientConfig) (*gossh.Client, error) {
	if c.DialTimeout > 0 {
		_ = conn.SetDeadline(time.Now().Add(c.DialTimeout))
	}
	cc, chans, reqs, err := gossh.NewClientConn(conn, addr, cfg)
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return gossh.NewClient(cc, chans, reqs), nil
}
