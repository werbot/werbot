package connectortest

import (
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway"
)

// maxDatagram is the largest probe: a full Ethernet UDP payload.
const maxDatagram = 1472

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
		if len(m.p) > len(buf) {
			return m.flow, 0, io.ErrShortBuffer // dropped, like channelDatagrams
		}
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

// Recv returns the next datagram the session sent to the client, or an error after wait.
func (p *DatagramPeer) Recv(wait time.Duration) (flow uint32, d []byte, err error) {
	select {
	case m := <-p.fromConn:
		return m.flow, m.p, nil
	case <-p.done:
		return 0, nil, net.ErrClosed
	case <-time.After(wait):
		return 0, nil, errors.New("no datagram within " + wait.String())
	}
}

// quiet is how long driveDatagrams waits for a reply nobody asked for.
const quiet = 100 * time.Millisecond

// driveDatagrams sends probes into flows 1 and 2, an empty datagram into flow 3 and
// a maximum-size one into flow 4. Each must come back once, in its own flow.
func driveDatagrams(t *testing.T, peer *DatagramPeer) {
	t.Helper()
	big := make([]byte, maxDatagram)
	for i := range big {
		big[i] = byte(i)
	}
	want := map[uint32]string{1: probe + "-1", 2: probe + "-2", 3: "", 4: string(big)}
	for flow, d := range want {
		peer.Send(flow, []byte(d))
	}
	got := map[uint32]string{}
	for range want {
		flow, d, err := peer.Recv(waitFor)
		require.NoError(t, err)
		got[flow] = string(d)
	}
	require.Equal(t, want, got, "a datagram came back in the wrong flow")
	flow, d, err := peer.Recv(quiet)
	require.Error(t, err, "an extra datagram came back in flow %d: %q", flow, d)
}
