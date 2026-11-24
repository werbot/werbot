//go:build linux || darwin

package vnet

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/netip"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/dns/dnsmessage"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

func ctxT(t *testing.T, d time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), d)
	t.Cleanup(cancel)
	return ctx
}

func dialT(t *testing.T, tnet *netstack.Net, to netip.Addr, port uint16) net.Conn {
	t.Helper()
	c, err := tnet.DialContextTCPAddrPort(ctxT(t, 5*time.Second), netip.AddrPortFrom(to, port))
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func echo(t *testing.T, c net.Conn, msg string) {
	t.Helper()
	_, err := c.Write([]byte(msg))
	require.NoError(t, err)
	got := make([]byte, len(msg))
	_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, err = io.ReadFull(c, got)
	require.NoError(t, err)
	require.Equal(t, msg, string(got))
}

func TestEngineTCP(t *testing.T) {
	d := newFakeDialer(t)
	_, tnet, addrs := startEngine(t, d, netip.Addr{}, res("pg", "stream"))
	echo(t, dialT(t, tnet, addrs["pg/tcp"], 5432), "ping")
	tcp, _ := d.calls()
	require.Equal(t, []string{"pg:5432"}, tcp)
}

func TestEngineTCPHalfClose(t *testing.T) {
	d := newFakeDialer(t)
	_, tnet, addrs := startEngine(t, d, netip.Addr{}, res("pg", "stream"))
	c := dialT(t, tnet, addrs["pg/tcp"], 5432)
	echo(t, c, "ping")
	require.NoError(t, c.(interface{ CloseWrite() error }).CloseWrite())
	rest, err := io.ReadAll(c) // the target saw EOF, answered and closed: both sides ended
	require.NoError(t, err)
	require.Equal(t, "bye", string(rest))
}

func TestEngineTCPDialFails(t *testing.T) {
	d := newFakeDialer(t)
	d.err = errDial
	_, tnet, addrs := startEngine(t, d, netip.Addr{}, res("pg", "stream"))
	start := time.Now()
	_, err := tnet.DialContextTCPAddrPort(ctxT(t, 5*time.Second), netip.AddrPortFrom(addrs["pg/tcp"], 5432))
	require.ErrorContains(t, err, "refused")
	require.Less(t, time.Since(start), time.Second)
}

func TestEngineUnknownAddress(t *testing.T) {
	d := newFakeDialer(t)
	_, tnet, addrs := startEngine(t, d, netip.Addr{}, res("pg", "stream"), res("dns", "datagram"))
	unknown := netip.MustParseAddr("100.64.0.50")
	require.NotContains(t, addrs, unknown)
	for _, to := range []netip.Addr{unknown, addrs["dns/udp"]} {
		_, err := tnet.DialContextTCPAddrPort(ctxT(t, 5*time.Second), netip.AddrPortFrom(to, 5432))
		require.ErrorContains(t, err, "refused", to)
	}
	tcp, _ := d.calls()
	require.Empty(t, tcp)
}

func TestEngineDNS(t *testing.T) {
	d := newFakeDialer(t)
	e, tnet, addrs := startEngine(t, d, netip.Addr{}, res("pg", "stream"))
	got, err := tnet.LookupContextHost(ctxT(t, 5*time.Second), "pg.acme.werbot")
	require.NoError(t, err)
	require.Equal(t, []string{addrs["pg/tcp"].String()}, got)

	c := dialT(t, tnet, DNSAddr(testPrefix), 53)
	qb := query(t, 9, q("pg.acme.werbot.", dnsmessage.TypeA))
	_, err = c.Write(append(binary.BigEndian.AppendUint16(nil, uint16(len(qb))), qb...)) //nolint:gosec // a test query is short
	require.NoError(t, err)
	var l [2]byte
	_, err = io.ReadFull(c, l[:])
	require.NoError(t, err)
	resp := make([]byte, binary.BigEndian.Uint16(l[:]))
	_, err = io.ReadFull(c, resp)
	require.NoError(t, err)
	want, _ := e.resolver.answer(qb)
	require.Equal(t, want, resp)
}

func TestEngineResourceRemoved(t *testing.T) {
	d := newFakeDialer(t)
	e, tnet, addrs := startEngine(t, d, netip.Addr{}, res("pg", "stream"), res("db", "stream"))
	active := dialT(t, tnet, addrs["pg/tcp"], 5432)
	echo(t, active, "ping")
	_, err := tnet.LookupContextHost(ctxT(t, 5*time.Second), "pg.acme.werbot")
	require.NoError(t, err)

	as, err := e.SetResources([]Resource{res("db", "stream")})
	require.NoError(t, err)
	require.Equal(t, []Assignment{{Name: "db", Addr: addrs["db/tcp"], Proto: "tcp"}}, as)

	_, err = tnet.LookupContextHost(ctxT(t, 5*time.Second), "pg.acme.werbot")
	require.ErrorContains(t, err, "no such host")
	_, err = tnet.DialContextTCPAddrPort(ctxT(t, 5*time.Second), netip.AddrPortFrom(addrs["pg/tcp"], 5432))
	require.ErrorContains(t, err, "refused")

	_ = active.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, err = active.Read(make([]byte, 8))
	require.Error(t, err)
	ne, ok := errors.AsType[net.Error](err)
	require.False(t, ok && ne.Timeout(), "the active connection must be closed, not idle")

	echo(t, dialT(t, tnet, addrs["db/tcp"], 5432), "still")
}

func TestEngineDropsForeignSource(t *testing.T) {
	d := newFakeDialer(t)
	_, tnet, addrs := startEngine(t, d, netip.MustParseAddr("100.64.0.100"), res("pg", "stream"))
	_, err := tnet.DialContextTCPAddrPort(ctxT(t, 500*time.Millisecond), netip.AddrPortFrom(addrs["pg/tcp"], 5432))
	require.Error(t, err)
	_, err = tnet.LookupContextHost(ctxT(t, 500*time.Millisecond), "pg.acme.werbot")
	require.Error(t, err)
	tcp, _ := d.calls()
	require.Empty(t, tcp)
}

func TestEngineNoGoroutineLeak(t *testing.T) {
	d := newFakeDialer(t)
	base := runtime.NumGoroutine()
	dev, tnet, err := netstack.CreateNetTUN([]netip.Addr{IfaceAddr(testPrefix)}, []netip.Addr{DNSAddr(testPrefix)}, 1500)
	require.NoError(t, err)
	e, err := newEngine(dev, testPrefix, "acme.werbot", d)
	require.NoError(t, err)
	as, err := e.SetResources([]Resource{res("pg", "stream")})
	require.NoError(t, err)
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			c, err := tnet.DialContextTCPAddrPort(t.Context(), netip.AddrPortFrom(as[0].Addr, 5432))
			if !assert.NoError(t, err) {
				return
			}
			_, err = c.Write([]byte("x"))
			assert.NoError(t, err)
			_, err = io.ReadFull(c, make([]byte, 1))
			assert.NoError(t, err)
		})
	}
	wg.Wait()
	require.NoError(t, e.Close())
	require.True(t, d.closed)
	// +1: assert.Eventually runs the condition in a goroutine of its own
	assert.Eventually(t, func() bool { return runtime.NumGoroutine() <= base+1 }, 10*time.Second, 50*time.Millisecond)
}
