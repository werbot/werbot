//go:build load

package load

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	tcpv1 "github.com/werbot/werbot/gen/werbot/connector/tcp/v1"
	udpv1 "github.com/werbot/werbot/gen/werbot/connector/udp/v1"
	connectorv1 "github.com/werbot/werbot/gen/werbot/connector/v1"
)

// The L1 mix of stage 6b (11 §9): TCP connections and UDP bindings through
// one werbot connect, beside the SSH sessions.
const (
	tcpChunk   = 1024 // bytes a second on each TCP connection
	udpPayload = 64   // bytes of the one datagram a second on each UDP binding
	udpBase    = 20000
)

// mixResult is one TCP connection or UDP binding of the mix. Err is a
// connection error, a wrong echo or no echo for longer than outputGap.
type mixResult struct {
	Err    error
	Echoed int64 // bytes that came back
}

// startMix starts the TCP and UDP echo targets of this process, creates
// "load-tcp" and "load-udp" on pool-a for them and one werbot connect with
// "load-tcp" and *udpBinds specs "<20000+i>:load-udp" (ruling R6). It returns
// the local TCP address and the local UDP addresses.
func startMix(t *testing.T) (tcp string, udp []string) {
	t.Helper()
	ip := bridgeIP(t)
	var specs []string
	if *tcpConns > 0 {
		port := tcpEcho(t, ip)
		st.AddPortResource(t, "load-tcp", &tcpv1.Config{Host: "host.docker.internal", Ports: []*connectorv1.PortRange{{Port: port}}})
		specs = append(specs, "load-tcp")
	}
	if *udpBinds > 0 {
		port := udpEcho(t, ip)
		st.AddPortResource(t, "load-udp", &udpv1.Config{Host: "host.docker.internal", Ports: []*connectorv1.PortRange{{Port: port}}})
		for i := range *udpBinds {
			p := strconv.Itoa(udpBase + i)
			specs = append(specs, p+":load-udp")
			udp = append(udp, "127.0.0.1:"+p)
		}
	}
	if len(specs) == 0 {
		return "", nil
	}
	return st.StartConnectAt(t, nginxAddr(t, 2222), "alice", specs...).Ports["load-tcp"], udp
}

// mixRun is a running mix: Steady closes once every connection has echoed (or
// failed, which Results then shows); Results arrives once all have stopped.
type mixRun struct {
	Steady  <-chan struct{}
	Results <-chan [2][]mixResult
	start   time.Time
}

// runMix keeps *tcpConns connections to tcp at tcpChunk bytes a second and a
// datagram a second on each address of udp until ctx ends; connections start
// at rate per second.
func runMix(ctx context.Context, tcp string, udp []string, rate int) *mixRun {
	out, steady := make(chan [2][]mixResult, 1), make(chan struct{})
	res := [2][]mixResult{make([]mixResult, *tcpConns), make([]mixResult, len(udp))}
	step := time.Second / time.Duration(max(rate, 1))
	var wg, warm sync.WaitGroup
	warm.Add(len(res[0]) + len(udp))
	for i := range res[0] {
		wg.Go(func() { res[0][i] = echoLoop(ctx, "tcp", tcp, tcpChunk, time.Duration(i)*step, warm.Done) })
	}
	for i, addr := range udp {
		wg.Go(func() { res[1][i] = echoLoop(ctx, "udp", addr, udpPayload, time.Duration(i)*step, warm.Done) })
	}
	go func() { warm.Wait(); close(steady) }()
	go func() { wg.Wait(); out <- res }()
	return &mixRun{Steady: steady, Results: out, start: time.Now()}
}

// waitSteady waits up to limit for Steady and returns the time since the mix started.
func (m *mixRun) waitSteady(limit time.Duration) (time.Duration, bool) {
	select {
	case <-m.Steady:
		return time.Since(m.start), true
	case <-time.After(limit):
		return time.Since(m.start), false
	}
}

// steadyRow is the "mix ramp" row: how long the mix took to reach steady
// state; a timeout fails it, so a mix that never settled is not a pass.
func steadyRow(took, limit time.Duration, ok bool) (string, bool) {
	if !ok {
		return "not within " + limit.String(), false
	}
	return took.Round(10 * time.Millisecond).String(), true
}

// echoLoop waits delay, then sends size random bytes through addr every
// second and expects them back within outputGap, until ctx ends. warm is
// called once: after the first echo, or when the loop ends without one.
func echoLoop(ctx context.Context, network, addr string, size int, delay time.Duration, warm func()) (r mixResult) {
	warm = sync.OnceFunc(warm)
	defer warm()
	select {
	case <-ctx.Done():
		return r // not started: Echoed 0 fails the row
	case <-time.After(delay):
	}
	c, err := (&net.Dialer{}).DialContext(ctx, network, addr)
	if err != nil {
		r.Err = fmt.Errorf("dial %s: %w", network, err)
		return r
	}
	defer c.Close()
	sent, got := make([]byte, size), make([]byte, size)
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		_, _ = rand.Read(sent) // never fails
		_ = c.SetDeadline(time.Now().Add(outputGap))
		if _, err := c.Write(sent); err != nil {
			r.Err = fmt.Errorf("%s write: %w", network, err)
			return r
		}
		if _, err := io.ReadFull(c, got); err != nil {
			r.Err = fmt.Errorf("%s echo after %d bytes: %w", network, r.Echoed, err)
			return r
		}
		if !bytes.Equal(sent, got) {
			r.Err = fmt.Errorf("%s echo after %d bytes: wrong data", network, r.Echoed)
			return r
		}
		r.Echoed += int64(size)
		warm()
		select {
		case <-ctx.Done():
			return r
		case <-tick.C:
		}
	}
}

// mixRow is the result of one kind of the mix; it passes when every
// connection echoed and none failed, so a mix that measured nothing fails.
func mixRow(res []mixResult) (string, bool) {
	var errs []error
	silent, bytes := 0, int64(0)
	for _, r := range res {
		if r.Err != nil {
			errs = append(errs, r.Err)
		}
		if r.Echoed == 0 {
			silent++
		}
		bytes += r.Echoed
	}
	result := fmt.Sprintf("%d of %d, %d silent, %s echoed", len(errs), len(res), silent, mb(bytes))
	if len(errs) > 0 {
		result += ", first: " + errors.Join(errs[:min(len(errs), 3)]...).Error()
	}
	return result, len(res) > 0 && len(errs) == 0 && silent == 0
}

// reportMix reports the TCP and UDP rows of the mix that ran.
func reportMix(t testing.TB, res [2][]mixResult) {
	t.Helper()
	for i, kind := range []string{"TCP", "UDP"} {
		if len(res[i]) == 0 {
			continue // -load.tcp=0 or -load.udp=0
		}
		result, pass := mixRow(res[i])
		report(t, "L1", kind+" via werbot connect: errors", "0", result, pass)
	}
}

// tcpEcho serves a TCP echo on ip and returns its port.
func tcpEcho(t testing.TB, ip string) uint32 {
	t.Helper()
	ln, err := net.Listen("tcp", net.JoinHostPort(ip, "0"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); _, _ = io.Copy(c, c) }()
		}
	}()
	return uint32(ln.Addr().(*net.TCPAddr).Port) //nolint:gosec // a TCP port fits uint32
}

// udpEcho serves a UDP echo on ip and returns its port.
func udpEcho(t testing.TB, ip string) uint32 {
	t.Helper()
	pc, err := net.ListenPacket("udp", net.JoinHostPort(ip, "0"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = pc.Close() })
	go func() {
		buf := make([]byte, 64<<10)
		for {
			n, from, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			_, _ = pc.WriteTo(buf[:n], from)
		}
	}()
	return uint32(pc.LocalAddr().(*net.UDPAddr).Port) //nolint:gosec // a UDP port fits uint32
}
