package connectortest

import (
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway"
)

type packet struct {
	flow uint32
	p    []byte
}

// DatagramPeer is the client end of the in-memory pair made by Datagrams.
type DatagramPeer struct {
	toConn, fromConn chan packet
	done             chan struct{}
}

type datagramConn struct {
	p         *DatagramPeer
	closeOnce func()
}

// Datagrams returns an in-memory pair: client goes to Session.Datagram, peer
// plays the user (Send a datagram into the session, Recv what the session sent).
func Datagrams() (client gateway.Datagrams, peer *DatagramPeer) {
	peer = &DatagramPeer{toConn: make(chan packet, 16), fromConn: make(chan packet, 16), done: make(chan struct{})}
	return datagramConn{peer, sync.OnceFunc(func() { close(peer.done) })}, peer
}

func (c datagramConn) Recv(buf []byte) (uint32, int, error) {
	select {
	case m := <-c.p.toConn:
		return m.flow, copy(buf, m.p), nil
	case <-c.p.done:
		return 0, 0, net.ErrClosed
	}
}

func (c datagramConn) Send(flow uint32, p []byte) error {
	select {
	case c.p.fromConn <- packet{flow, append([]byte(nil), p...)}:
		return nil
	case <-c.p.done:
		return net.ErrClosed
	}
}

func (c datagramConn) Close() error { c.closeOnce(); return nil }

// Send puts a datagram of flow into the session.
func (p *DatagramPeer) Send(flow uint32, d []byte) {
	select {
	case p.toConn <- packet{flow, append([]byte(nil), d...)}:
	case <-p.done:
	}
}

// Recv returns the next datagram the session sent to the client.
func (p *DatagramPeer) Recv() (flow uint32, d []byte, err error) {
	select {
	case m := <-p.fromConn:
		return m.flow, m.p, nil
	case <-p.done:
		return 0, nil, net.ErrClosed
	case <-time.After(waitFor):
		return 0, nil, errors.New("no datagram within " + waitFor.String())
	}
}

// driveDatagrams sends one probe into each of two flows and expects each back in its own flow.
func driveDatagrams(t *testing.T, peer *DatagramPeer) {
	t.Helper()
	for flow := uint32(1); flow <= 2; flow++ {
		peer.Send(flow, []byte(probe+"-"+string(rune('0'+flow))))
	}
	got := map[uint32]string{}
	for range 2 {
		flow, d, err := peer.Recv()
		require.NoError(t, err)
		got[flow] = string(d)
	}
	require.Equal(t, map[uint32]string{1: probe + "-1", 2: probe + "-2"}, got, "a datagram came back in the wrong flow")
}
