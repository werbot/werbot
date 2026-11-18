//go:build load

package load

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	gliderssh "github.com/gliderlabs/ssh"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/test/cluster"
)

// targetPassword is the password of the load target's login "load" (test-only).
const targetPassword = "load-password"

// startTarget runs the SSH target of L1/L3/L4 in the test process. It listens on
// the Docker bridge gateway address, where host.docker.internal of the gateways
// points (host-gateway), not on 0.0.0.0: no LAN exposure of a password login
// (ruling G12). A shell writes "ready\r\n", then outputPerSec bytes of printable
// text each second, until the client types exit or closes. It lives as long as the test process.
func startTarget(t testing.TB) (port int, setOutput func(bytesPerSec int)) {
	t.Helper()
	ln, err := net.Listen("tcp", net.JoinHostPort(bridgeIP(t), "0"))
	require.NoError(t, err)
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := gossh.NewSignerFromKey(key)
	require.NoError(t, err)
	var perSec atomic.Int64
	srv := &gliderssh.Server{
		HostSigners: []gliderssh.Signer{signer},
		PasswordHandler: func(ctx gliderssh.Context, password string) bool {
			return ctx.User() == "load" && password == targetPassword
		},
		Handler: func(s gliderssh.Session) { shell(s, &perSec) },
	}
	go func() { _ = srv.Serve(ln) }()
	return ln.Addr().(*net.TCPAddr).Port, func(n int) { perSec.Store(int64(n)) }
}

// bridgeIP is the Docker bridge gateway address: host.docker.internal of the
// gateways (host-gateway). Targets of this process listen only there (ruling G12).
func bridgeIP(t testing.TB) string {
	t.Helper()
	ip, err := exec.CommandContext(context.Background(), "docker", "network", "inspect", "bridge", "-f", "{{(index .IPAM.Config 0).Gateway}}").Output()
	require.NoError(t, err, "docker bridge gateway address")
	return strings.TrimSpace(string(ip))
}

// shell serves one session: "ready", then perSec bytes a second until "exit" or close.
func shell(s gliderssh.Session, perSec *atomic.Int64) {
	quit := make(chan struct{})
	go func() {
		defer close(quit)
		var in []byte
		buf := make([]byte, 256)
		for {
			n, err := s.Read(buf)
			in = append(in, buf[:n]...)
			if err != nil || bytes.Contains(in, []byte("exit")) {
				return
			}
		}
	}()
	if _, err := io.WriteString(s, "ready\r\n"); err != nil {
		return
	}
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-quit:
			_ = s.Exit(0)
			return
		case <-s.Context().Done():
			return
		case <-tick.C:
			if n := perSec.Load(); n > 0 {
				if _, err := s.Write(text(int(n))); err != nil {
					return
				}
			}
		}
	}
}

// text is n bytes of random printable ASCII in lines of 76 characters
// ("\r\n" after each); random, so recordings do not compress away.
func text(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b) // never fails
	for i := range b {
		switch i % 78 {
		case 76:
			b[i] = '\r'
		case 77:
			b[i] = '\n'
		default:
			b[i] = ' ' + b[i]%95
		}
	}
	return b
}

// nginxAddr is port of the nginx container on its stand network address: the
// load tests dial it directly, not through the docker-proxy of the published
// port (Linux only: the host reaches bridge networks there).
func nginxAddr(port int) string {
	return net.JoinHostPort(cluster.IPNginx, strconv.Itoa(port))
}
