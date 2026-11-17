package connectortest_test

import (
	"context"
	"io"
	"net"
	"sync"

	"github.com/werbot/werbot/internal/gateway"
)

func echoTCP(l net.Listener) {
	for {
		c, err := l.Accept()
		if err != nil {
			return
		}
		go func() { defer c.Close(); _, _ = io.Copy(c, c) }()
	}
}

func echoUDP(pc net.PacketConn) {
	buf := make([]byte, 65535)
	for {
		n, from, err := pc.ReadFrom(buf)
		if err != nil {
			return
		}
		_, _ = pc.WriteTo(buf[:n], from)
	}
}

// tcpPipe is a fake TargetDialer: it pipes the stream to s.Target through s.Dial.
type tcpPipe struct{}

func (tcpPipe) Type() string          { return "tcppipe" }
func (tcpPipe) Kinds() []gateway.Kind { return []gateway.Kind{gateway.KindStream} }
func (tcpPipe) DialsTarget()          {}
func (tcpPipe) Serve(ctx context.Context, s *gateway.Session) error {
	defer s.Stream.Close()
	conn, err := s.Dial(ctx, "tcp", s.Target)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer context.AfterFunc(ctx, func() { _ = s.Stream.Close(); _ = conn.Close() })()
	go func() { _, _ = io.Copy(conn, s.Stream) }()
	_, _ = io.Copy(s.Stream, conn)
	return nil
}

// udpEcho is a fake TargetDialer: one UDP socket per flow, dialed through s.Dial.
type udpEcho struct{}

func (udpEcho) Type() string          { return "udpecho" }
func (udpEcho) Kinds() []gateway.Kind { return []gateway.Kind{gateway.KindDatagram} }
func (udpEcho) DialsTarget()          {}
func (udpEcho) Serve(ctx context.Context, s *gateway.Session) error {
	var wg sync.WaitGroup
	conns := map[uint32]net.Conn{}
	defer wg.Wait()
	defer func() {
		for _, c := range conns {
			_ = c.Close()
		}
	}()
	defer s.Datagram.Close()
	defer context.AfterFunc(ctx, func() { _ = s.Datagram.Close() })()
	buf := make([]byte, 65535)
	for {
		flow, n, err := s.Datagram.Recv(buf)
		if err != nil {
			return nil //nolint:nilerr // the client end closed: a normal end
		}
		c, ok := conns[flow]
		if !ok {
			if c, err = s.Dial(ctx, "udp", s.Target); err != nil {
				return err
			}
			conns[flow] = c
			wg.Go(func() {
				reply := make([]byte, 65535)
				for {
					m, err := c.Read(reply)
					if err != nil {
						return
					}
					_ = s.Datagram.Send(flow, reply[:m])
				}
			})
		}
		if _, err := c.Write(buf[:n]); err != nil {
			return err
		}
	}
}
