//go:build linux || darwin

package vnet

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
)

func TestEngineUDPFlows(t *testing.T) {
	old := flowIdle
	flowIdle = 2 * time.Second
	t.Cleanup(func() { flowIdle = old })

	d := newFakeDialer(t)
	_, tnet, addrs := startEngine(t, d, netip.Addr{}, res("dns", "datagram"))
	to := netip.AddrPortFrom(addrs["dns/udp"], 53)
	socks := make([]*gonet.UDPConn, maxFlows+1)
	for i := range socks {
		c, err := tnet.DialUDPAddrPort(netip.AddrPort{}, to)
		require.NoError(t, err)
		t.Cleanup(func() { _ = c.Close() })
		socks[i] = c
	}
	exchange := func(i int) (string, error) {
		if _, err := socks[i].Write(fmt.Appendf(nil, "q%d", i)); err != nil {
			return "", err
		}
		_ = socks[i].SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		buf := make([]byte, 64)
		n, err := socks[i].Read(buf)
		return string(buf[:n]), err
	}

	for i := range maxFlows { // every socket is a flow; replies come back to their own socket
		got, err := exchange(i)
		require.NoError(t, err, i)
		require.Equal(t, fmt.Sprintf("re:q%d", i), got)
	}
	_, err := exchange(maxFlows)
	require.Error(t, err, "a flow over the limit is dropped")
	_, dgram := d.calls()
	require.Equal(t, []string{"dns:53"}, dgram, "one channel for all flows of the resource port")

	// idle flows end, and the channel with them
	require.Eventually(t, func() bool { return d.channelsClosed() == 1 }, 3*flowIdle, 50*time.Millisecond)
	got, err := exchange(maxFlows)
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("re:q%d", maxFlows), got)
	_, dgram = d.calls()
	require.Len(t, dgram, 2)
}
