//go:build linux || darwin

package vnet

import (
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.zx2c4.com/wireguard/tun/netstack"

	"github.com/werbot/werbot/internal/platform/sshdgram"
)

var testPrefix = netip.MustParsePrefix(DefaultPrefix)

func res(name string, kinds ...string) Resource {
	return Resource{ID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(name)), Name: name, Kinds: kinds}
}

// fakeDialer stands for the gateway: TCP goes to a loopback target that
// echoes until EOF, then writes "bye" and closes; datagram channels echo every
// frame with "re:" before the payload.
type fakeDialer struct {
	target net.Listener
	err    error

	mu          sync.Mutex
	tcp, dgram  []string
	dgramClosed int
	closed      bool
}

func newFakeDialer(t *testing.T) *fakeDialer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				_, _ = io.Copy(struct{ io.Writer }{c}, c)
				_, _ = c.Write([]byte("bye"))
				_ = c.Close()
			}()
		}
	}()
	return &fakeDialer{target: ln}
}

func (f *fakeDialer) DialTCP(_ context.Context, r Resource, port uint16) (io.ReadWriteCloser, error) {
	f.mu.Lock()
	f.tcp = append(f.tcp, r.Name+":"+strconv.Itoa(int(port)))
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return net.Dial("tcp", f.target.Addr().String())
}

func (f *fakeDialer) DialDatagram(_ context.Context, r Resource, port uint16) (io.ReadWriteCloser, error) {
	f.mu.Lock()
	f.dgram = append(f.dgram, r.Name+":"+strconv.Itoa(int(port)))
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	ours, theirs := net.Pipe()
	go func() {
		buf := make([]byte, sshdgram.MaxPayload)
		for {
			flow, n, err := sshdgram.ReadFrame(theirs, buf)
			if err != nil {
				_ = theirs.Close()
				f.mu.Lock()
				f.dgramClosed++
				f.mu.Unlock()
				return
			}
			if sshdgram.WriteFrame(theirs, flow, append([]byte("re:"), buf[:n]...)) != nil {
				continue
			}
		}
	}()
	return ours, nil
}

func (f *fakeDialer) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *fakeDialer) calls() (tcp, dgram []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.tcp...), append([]string(nil), f.dgram...)
}

func (f *fakeDialer) channelsClosed() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.dgramClosed
}

// startEngine runs an engine whose TUN is a netstack client with the given
// address (the interface address when none) and returns the client and the
// addresses by "name/proto".
func startEngine(t *testing.T, d Dialer, src netip.Addr, rs ...Resource) (*engine, *netstack.Net, map[string]netip.Addr) {
	t.Helper()
	if !src.IsValid() {
		src = IfaceAddr(testPrefix)
	}
	dev, tnet, err := netstack.CreateNetTUN([]netip.Addr{src}, []netip.Addr{DNSAddr(testPrefix)}, 1500)
	require.NoError(t, err)
	e, err := newEngine(dev, testPrefix, "acme.werbot", d)
	require.NoError(t, err)
	t.Cleanup(func() { _ = e.Close() })
	as, err := e.SetResources(rs)
	require.NoError(t, err)
	addrs := map[string]netip.Addr{}
	for _, a := range as {
		addrs[a.Name+"/"+a.Proto] = a.Addr
	}
	return e, tnet, addrs
}

var errDial = errors.New("gateway refused the channel")
