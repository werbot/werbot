package udpconnector_test

import (
	"errors"
	"fmt"
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	_ "github.com/werbot/werbot/internal/connector/udp" // the udp resource type
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/connectortest"
	"github.com/werbot/werbot/internal/gateway/udpconnector"
)

func TestContract(t *testing.T) {
	registered, ok := gateway.LookupConnector("udp")
	require.True(t, ok)
	require.Equal(t, udpconnector.Connector{}, registered)
	addr := echoUDP(t, "")
	connectortest.Run(t, udpconnector.Connector{}, connectortest.Target{
		Resource: resource(addr, ""), Secret: gateway.Secret{Kind: "none", Value: []byte("werbot-udp-secret")},
		Datagram: addr,
	})
}

func TestDialsTarget(t *testing.T) {
	var c gateway.Connector = udpconnector.Connector{}
	_, ok := c.(gateway.TargetDialer)
	require.True(t, ok)
}

func TestUDPFlowLimit(t *testing.T) {
	addr := echoUDP(t, "")
	d := newDgrams()
	h := start(t, addr, "", d)
	for f := byte(1); f <= 33; f++ {
		d.send(f, []byte{'a', f})
	}
	got := map[byte]bool{}
	for range 32 {
		f, p := d.recv(t)
		require.Equal(t, string([]byte{'a', f}), p)
		got[f] = true
	}
	require.Len(t, got, 32)
	require.False(t, got[33], "flow 33 was served")
	require.Eventually(t, func() bool { return h.metric(t, "flows") == "1" }, waitFor, 10*time.Millisecond)
	d.send(1, []byte("alive")) // the session lives
	f, p := d.recv(t)
	require.Equal(t, byte(1), f)
	require.Equal(t, "alive", p)
	h.finish(t)
}

// A flow without traffic for flow_idle loses its socket; its next datagram opens a new one.
func TestUDPFlowReopens(t *testing.T) {
	addr := echoUDP(t, "")
	d := newDgrams()
	h := start(t, addr, "1s", d)
	d.send(7, []byte("one"))
	_, p := d.recv(t)
	require.Equal(t, "one", p)
	select {
	case <-h.closed:
	case <-time.After(waitFor):
		t.Fatal("the idle flow was not closed")
	}
	d.send(7, []byte("two"))
	_, p = d.recv(t)
	require.Equal(t, "two", p)
	require.EqualValues(t, 2, h.dials.Load())
	h.finish(t)
}

func TestUDPOversize(t *testing.T) {
	addr := echoUDP(t, "")
	d := newDgrams()
	h := start(t, addr, "", d)
	d.send(1, make([]byte, 9000)) // from the client
	d.send(1, make([]byte, 100))
	_, p := d.recv(t)
	require.Len(t, p, 100)
	require.Eventually(t, func() bool { return h.metric(t, "size") == "1" }, waitFor, 5*time.Millisecond)
	h.finish(t)
}

// A target reply over 8192 bytes is dropped, not cut; the next reply arrives.
func TestUDPOversizeReply(t *testing.T) {
	d := newDgrams()
	h := start(t, bigUDP(t), "", d)
	d.send(1, []byte("q"))
	_, p := d.recv(t)
	require.Len(t, p, 100)
	require.Eventually(t, func() bool { return h.metric(t, "size") == "1" }, waitFor, 5*time.Millisecond)
	h.finish(t)
}

func TestUDPPortUnreachable(t *testing.T) {
	probe, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	addr := probe.LocalAddr().String()
	require.NoError(t, probe.Close()) // a closed port
	d := newDgrams()
	h := start(t, addr, "", d)
	d.send(1, []byte("lost"))
	require.Eventually(t, func() bool { return h.dials.Load() == 1 }, waitFor, 5*time.Millisecond)
	echoUDP(t, addr) // the target starts
	got := make(chan string, 1)
	require.Eventually(t, func() bool {
		d.send(1, []byte("hello"))
		select {
		case m := <-d.out:
			got <- string(m[1:])
			return true
		case <-time.After(50 * time.Millisecond):
			return false
		}
	}, waitFor, 10*time.Millisecond)
	require.Equal(t, "hello", <-got)
	require.EqualValues(t, 1, h.dials.Load(), "ECONNREFUSED must not close the flow")
	h.finish(t)
}

// A client that stops reading blocks Send; Serve still returns on cancel and the Recv loop keeps going.
func TestUDPBlockedSend(t *testing.T) {
	addr := echoUDP(t, "")
	d := newDgrams()
	d.block = true
	h := start(t, addr, "", d)
	d.send(1, []byte("a"))
	for i := range 3 { // Recv still takes datagrams of other flows while a Send is stuck
		d.send(byte(2+i), []byte("b"))
	}
	require.Eventually(t, func() bool { return len(d.in) == 0 }, waitFor, 5*time.Millisecond)
	h.finish(t)
}

// A dial error other than ErrTargetNotAllowed drops that datagram only; the next one retries.
func TestUDPDialError(t *testing.T) {
	addr := echoUDP(t, "")
	d := newDgrams()
	h := start(t, addr, "", d)
	h.failDial(errors.New("lookup failed"))
	d.send(1, []byte("lost"))
	require.Eventually(t, func() bool { return h.dials.Load() == 1 }, waitFor, 5*time.Millisecond)
	h.failDial(nil)
	require.Eventually(t, func() bool { // the first datagram may still be in flight
		d.send(1, []byte("ok"))
		select {
		case m := <-d.out:
			return string(m[1:]) == "ok"
		case <-time.After(50 * time.Millisecond):
			return false
		}
	}, waitFor, 10*time.Millisecond)
	h.finish(t)
}

func TestUDPTargetNotAllowed(t *testing.T) {
	d := newDgrams()
	h := start(t, echoUDP(t, ""), "", d)
	h.failDial(gateway.ErrTargetNotAllowed)
	d.send(1, []byte("x"))
	select {
	case err := <-h.served:
		require.ErrorIs(t, err, gateway.ErrTargetNotAllowed)
	case <-time.After(waitFor):
		t.Fatal("Serve did not return")
	}
}

// 32 flows of a session cost under 512 KiB: 8 KiB reader buffer, goroutine stack and socket each.
// ponytail: the pooled per-flow buffer dominates; a shared reader (one goroutine for all flows) would cut it.
// Skipped under -race: the race runtime inflates stacks and heap beyond a stable bound.
func TestUDPMemory(t *testing.T) {
	if raceEnabled {
		t.Skip("memory bound is not stable under -race")
	}
	const sessions, perSession = 8, 512 << 10
	addr := echoUDP(t, "") // its buffer is allocated before the snapshot
	var before, after runtime.MemStats
	runtime.GC()
	runtime.GC() // the second GC clears the pool victim cache
	runtime.ReadMemStats(&before)
	hs := make([]*harness, sessions)
	ds := make([]*dgrams, sessions)
	for i := range hs {
		ds[i] = newDgrams()
		hs[i] = start(t, addr, "", ds[i])
		for f := byte(1); f <= 32; f++ {
			ds[i].send(f, []byte("x"))
		}
	}
	for _, d := range ds {
		for range 32 {
			d.recv(t)
		}
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	grown := (int64(after.HeapAlloc+after.StackInuse) - int64(before.HeapAlloc+before.StackInuse)) / sessions //nolint:gosec // far below MaxInt64
	t.Logf("per session with 32 flows: %d bytes", grown)
	require.Less(t, grown, int64(perSession))
	for _, h := range hs {
		h.finish(t)
	}
}

type panicDgrams struct{ *dgrams }

func (panicDgrams) Recv([]byte) (uint32, int, error) { panic("fault injected") }

func TestUDPPanic(t *testing.T) {
	s := &gateway.Session{Resource: resource("127.0.0.1:9", ""), Datagram: panicDgrams{newDgrams()}, Target: "127.0.0.1:9",
		Secret: gateway.Secret{Value: []byte("udp-secret-value")}}
	var p any
	func() {
		defer func() { p = recover() }()
		_ = udpconnector.Connector{}.Serve(t.Context(), s)
	}()
	require.Contains(t, fmt.Sprint(p), "fault injected")
}
