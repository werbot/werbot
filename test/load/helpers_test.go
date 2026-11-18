//go:build load

package load

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	gliderssh "github.com/gliderlabs/ssh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/test/cluster"
)

// Pure helpers; they run on any OS: go test -tags load -short ./test/load/

func TestBudgets(t *testing.T) {
	assert.Equal(t, int64(840_000_000), gatewayBudget(2500)) // 10 §10.1
	assert.Equal(t, int64(200_000_000), gatewayBudget(0))
	assert.Equal(t, int64(300_000_000+64_000*202), serverBudget(202))
}

func TestP99(t *testing.T) {
	assert.Equal(t, time.Duration(0), p99(nil))
	assert.Equal(t, 7*time.Second, p99([]time.Duration{7 * time.Second}))
	d := make([]time.Duration, 0, 200)
	for i := 200; i >= 1; i-- { // unsorted input, 1..200 ms
		d = append(d, time.Duration(i)*time.Millisecond)
	}
	assert.Equal(t, 198*time.Millisecond, p99(d)) // nearest rank: ceil(0.99*200) = 198
	assert.Equal(t, 200*time.Millisecond, d[0], "p99 must not reorder its input")
	d = d[:101] // 200..100 ms: ceil(0.99*101) = 100 -> 100th smallest = 199 ms
	assert.Equal(t, 199*time.Millisecond, p99(d))
}

func TestParseVMRSS(t *testing.T) {
	status := "Name:\twerbot\nVmPeak:\t  900000 kB\nVmRSS:\t  412345 kB\nRssAnon:\t  400000 kB\n"
	v, err := parseVMRSS(strings.NewReader(status))
	require.NoError(t, err)
	assert.Equal(t, int64(412345*1024), v)
	_, err = parseVMRSS(strings.NewReader("Name:\tzombie\nState:\tZ (zombie)\n"))
	assert.Error(t, err)
}

func TestParseListen(t *testing.T) {
	tcp := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0B00007F:8C4F 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 1 1 0000000000000000 100 0 0 10 0
   1: 00000000:08AE 00000000:0000 0A 00000000:00000000 00:00000000 00000000 65532        0 2 1 0000000000000000 100 0 0 10 0
   2: 1500E70A:08AE 0100E70A:C350 01 00000000:00000000 00:00000000 00000000 65532        0 3 1 0000000000000000 20 4 30 10 -1
`
	tcp6 := `  sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000000000000000000000000000:2382 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000 65532        0 4 1 0000000000000000 100 0 0 10 0
   1: 00000000000000000000000000000000:08AE 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000 65532        0 5 1 0000000000000000 100 0 0 10 0
`
	got, err := parseListen(strings.NewReader(tcp), strings.NewReader(tcp6))
	require.NoError(t, err)
	// 127.0.0.11 is Docker's embedded DNS, not the gateway; established sockets do not count
	assert.Equal(t, []uint16{2222, 9090}, got)
}

func TestResultLine(t *testing.T) {
	assert.Equal(t, "RESULT | L1 | connect p99 | ≤ 1s | 412ms | PASS", resultLine("L1", "connect p99", "≤ 1s", "412ms", true))
	assert.Equal(t, "RESULT | L4 | listeners | 2222, 9090 | 22 | FAIL", resultLine("L4", "listeners", "2222, 9090", "22", false))
}

func TestText(t *testing.T) {
	b := text(1024)
	require.Len(t, b, 1024)
	for i, c := range b {
		switch i % 78 {
		case 76:
			assert.Equal(t, byte('\r'), c, "byte %d", i)
		case 77:
			assert.Equal(t, byte('\n'), c, "byte %d", i)
		default:
			assert.True(t, c >= ' ' && c <= '~', "byte %d = %q is not printable", i, c)
		}
	}
}

// TestSessionAgainstShell runs the load client against the target shell over
// plain SSH (no gateway): ready, output during hold, exit closes the session.
func TestSessionAgainstShell(t *testing.T) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	client, err := gossh.NewSignerFromKey(key)
	require.NoError(t, err)
	_, key, err = ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	host, err := gossh.NewSignerFromKey(key)
	require.NoError(t, err)
	var perSec atomic.Int64
	perSec.Store(1024)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := &gliderssh.Server{
		HostSigners:      []gliderssh.Signer{host},
		PublicKeyHandler: func(gliderssh.Context, gliderssh.PublicKey) bool { return true },
		Handler:          func(s gliderssh.Session) { shell(s, &perSec) },
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	start := time.Now()
	r := session(t.Context(), &cluster.Stand{Key: client, HostKey: host.PublicKey()}, ln.Addr().String(), 2500*time.Millisecond, true)
	require.NoError(t, r.Err)
	assert.Positive(t, r.Connect)
	assert.GreaterOrEqual(t, r.Bytes, int64(len("ready\r\n")+2*1024), "two ticks of output during the hold")
	assert.Less(t, time.Since(start), 2500*time.Millisecond+exitWait/2, "exit closes the session")
}

func TestPGRow(t *testing.T) {
	both := func(a, b int64) sample { return sample{PGConns: map[string]int64{"server1": a, "server2": b}} }
	got, pass := pgRow([]sample{both(5, 7), both(22, 3)})
	assert.True(t, pass)
	assert.Equal(t, "max 22 / 7", got)
	_, pass = pgRow([]sample{both(23, 3)})
	assert.False(t, pass, "over pool_max_conns + 2")
	got, pass = pgRow([]sample{{PGConns: map[string]int64{"server2": 4}}, {PGConns: map[string]int64{}}})
	assert.False(t, pass, "server1 never measured")
	assert.Equal(t, "no samples (server1 0, server2 1)", got)
	_, pass = pgRow(nil)
	assert.False(t, pass)
}

func TestActiveRow(t *testing.T) {
	samples := []sample{{Active: map[string]int64{"gw1": 1200}}, {Active: map[string]int64{"gw1": 2500}}, {Active: map[string]int64{}}}
	got, pass := activeRow(samples, "gw1", 2500)
	assert.True(t, pass)
	assert.Equal(t, "2500", got)
	got, pass = activeRow(samples, "gw2", 2500)
	assert.False(t, pass, "no load on gw2")
	assert.Equal(t, "0", got)
}

func TestMixRow(t *testing.T) {
	got, pass := mixRow([]mixResult{{Echoed: 2048}, {Echoed: 1024}})
	assert.True(t, pass)
	assert.Equal(t, "0 of 2, 0 silent, 0.0 MB echoed", got)
	_, pass = mixRow([]mixResult{{Echoed: 2048}, {}})
	assert.False(t, pass, "a connection that never echoed")
	got, pass = mixRow([]mixResult{{Echoed: 64, Err: errors.New("udp echo after 64 bytes: i/o timeout")}})
	assert.False(t, pass)
	assert.Equal(t, "1 of 1, 0 silent, 0.0 MB echoed, first: udp echo after 64 bytes: i/o timeout", got)
	_, pass = mixRow(nil)
	assert.False(t, pass, "nothing measured")
}

// TestEchoLoop runs the mix client against the echo targets of this process (no gateway).
func TestEchoLoop(t *testing.T) {
	for _, network := range []string{"tcp", "udp"} {
		echo := map[string]func(testing.TB, string) uint32{"tcp": tcpEcho, "udp": udpEcho}[network]
		addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(echo(t, "127.0.0.1"))))
		ctx, cancel := context.WithTimeout(t.Context(), 1500*time.Millisecond)
		r := echoLoop(ctx, network, addr, 64, 0)
		cancel()
		require.NoError(t, r.Err, network)
		assert.Equal(t, int64(2*64), r.Echoed, "%s: two ticks in 1.5 s", network)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	assert.Equal(t, mixResult{}, echoLoop(ctx, "tcp", "127.0.0.1:1", 64, time.Hour), "not started")
}
