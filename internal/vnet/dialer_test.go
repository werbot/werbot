//go:build linux || darwin

package vnet

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/connectcli"
	"github.com/werbot/werbot/internal/platform/sshdgram"
)

// sshGateway is a test gateway: public key of the user, then one
// keyboard-interactive code; it echoes direct-tcpip and datagram channels.
type sshGateway struct {
	ln      net.Listener
	hostKey string // authorized_keys line

	mu     sync.Mutex
	conns  []*ssh.ServerConn
	opened []string // user host:port of every channel
}

func startSSHGateway(t *testing.T, user ssh.PublicKey) *sshGateway {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	hk, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)
	conf := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, k ssh.PublicKey) (*ssh.Permissions, error) {
			if string(k.Marshal()) != string(user.Marshal()) {
				return nil, errors.New("unknown key")
			}
			return nil, &ssh.PartialSuccessError{Next: ssh.ServerAuthCallbacks{
				KeyboardInteractiveCallback: func(_ ssh.ConnMetadata, ch ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
					a, err := ch("", "", []string{"Code: "}, []bool{false})
					if err != nil || len(a) != 1 || a[0] != "123456" {
						return nil, errors.New("wrong code")
					}
					return nil, nil
				},
			}}
		},
	}
	conf.AddHostKey(hk)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	g := &sshGateway{ln: ln, hostKey: string(ssh.MarshalAuthorizedKey(hk.PublicKey()))}
	t.Cleanup(func() { _ = ln.Close(); g.drop() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go g.serve(c, conf)
		}
	}()
	return g
}

func (g *sshGateway) serve(c net.Conn, conf *ssh.ServerConfig) {
	sc, chans, reqs, err := ssh.NewServerConn(c, conf)
	if err != nil {
		_ = c.Close()
		return
	}
	g.mu.Lock()
	g.conns = append(g.conns, sc)
	g.mu.Unlock()
	go ssh.DiscardRequests(reqs)
	for nc := range chans {
		var d sshdgram.OpenData
		if ssh.Unmarshal(nc.ExtraData(), &d) != nil {
			_ = nc.Reject(ssh.ConnectionFailed, "bad open data")
			continue
		}
		g.mu.Lock()
		g.opened = append(g.opened, nc.ChannelType()+" "+sc.User()+" "+net.JoinHostPort(d.Host, strconv.FormatUint(uint64(d.Port), 10)))
		g.mu.Unlock()
		ch, creqs, err := nc.Accept()
		if err != nil {
			continue
		}
		go ssh.DiscardRequests(creqs)
		go func() { _, _ = io.Copy(ch, ch); _ = ch.Close() }()
	}
}

func (g *sshGateway) count() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.conns)
}

func (g *sshGateway) drop() {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, c := range g.conns {
		_ = c.Close()
	}
}

func testDialer(t *testing.T) (*gatewayDialer, *sshGateway) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)
	g := startSSHGateway(t, signer.PublicKey())
	d := newGatewayDialer("alice", func(context.Context) (ssh.Signer, error) { return signer, nil }, nil)
	d.kbd, d.out = connectcli.KeyboardInteractive(nil, strings.NewReader(strings.Repeat("123456\n", 4)), io.Discard), io.Discard
	t.Cleanup(func() { _ = d.Close() })
	return d, g
}

func roundTrip(t *testing.T, c io.ReadWriter, msg string) {
	t.Helper()
	_, err := c.Write([]byte(msg))
	require.NoError(t, err)
	got := make([]byte, len(msg))
	_, err = io.ReadFull(c, got)
	require.NoError(t, err)
	require.Equal(t, msg, string(got))
}

func TestGatewayDialerReusesClient(t *testing.T) {
	d, g := testDialer(t)
	pg := res("pg", "stream", "datagram")
	pg.SSHAddress, pg.HostKey = g.ln.Addr().String(), g.hostKey
	ctx := ctxT(t, 10*time.Second)

	for range 3 {
		c, err := d.DialTCP(ctx, pg, 5432)
		require.NoError(t, err)
		roundTrip(t, c, "ping")
		require.NoError(t, c.Close())
	}
	dc, err := d.DialDatagram(ctx, pg, 53)
	require.NoError(t, err)
	require.NoError(t, sshdgram.WriteFrame(dc, 7, []byte("q")))
	buf := make([]byte, 8)
	flow, n, err := sshdgram.ReadFrame(dc, buf)
	require.NoError(t, err)
	require.Equal(t, uint32(7), flow)
	require.Equal(t, "q", string(buf[:n]))
	require.NoError(t, dc.Close())
	require.Equal(t, 1, g.count(), "one SSH connection per resource")

	g.drop() // the next dial connects again
	c, err := d.DialTCP(ctx, pg, 5432)
	require.NoError(t, err)
	roundTrip(t, c, "again")
	require.Equal(t, 2, g.count())

	g.mu.Lock()
	require.Equal(t, "direct-tcpip alice+pg pg:5432", g.opened[0])
	require.Equal(t, sshdgram.ChannelType+" alice+pg pg:53", g.opened[3])
	g.mu.Unlock()
}

func TestGatewayDialerWrongHostKey(t *testing.T) {
	d, g := testDialer(t)
	other := startSSHGateway(t, nil)
	pg := res("pg", "stream")
	pg.SSHAddress, pg.HostKey = g.ln.Addr().String(), other.hostKey
	_, err := d.DialTCP(ctxT(t, 10*time.Second), pg, 5432)
	require.ErrorContains(t, err, "host key")
	require.Zero(t, g.count())
}

func TestGatewayDialerClosed(t *testing.T) {
	d, g := testDialer(t)
	pg := res("pg", "stream")
	pg.SSHAddress, pg.HostKey = g.ln.Addr().String(), g.hostKey
	c, err := d.DialTCP(ctxT(t, 10*time.Second), pg, 5432)
	require.NoError(t, err)
	require.NoError(t, d.Close())
	_, err = c.Read(make([]byte, 1))
	require.Error(t, err, "Close ends the clients and their channels")
	_, err = d.DialTCP(ctxT(t, 10*time.Second), pg, 5432)
	require.ErrorIs(t, err, net.ErrClosed)
}
