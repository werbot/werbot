package gateway

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func (d *dialLog) setFail(addr string, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.fail[addr] = err
}

// nextPass advances the fake clock until the prober dialed again, then lets the pass finish.
func nextPass(d *dialLog) {
	for {
		time.Sleep(time.Second)
		synctest.Wait()
		d.mu.Lock()
		n := len(d.addrs)
		d.mu.Unlock()
		if n > 0 {
			return
		}
	}
}

func TestHealthReportsEveryPass(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		d := &dialLog{fail: map[string]error{}}
		h, reg := healthEnv(d.dial, hres(1, "hprobe", "10.0.0.1"))
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		go func() { defer close(done); h.run(ctx, uid(0), func() []uuid.UUID { return nil }) }()
		defer func() { cancel(); <-done }()

		synctest.Wait() // the first pass runs at once
		require.Len(t, d.take(), 2)
		first := h.take()
		require.Len(t, first, 1)
		require.True(t, first[0].GetOk())
		require.NotNil(t, first[0].GetCheckedAt())
		require.Empty(t, h.take(), "take empties the queue")

		nextPass(d)
		require.Len(t, d.take(), 2)
		again := h.take()
		require.Len(t, again, 1, "an unchanged result is queued again")
		require.True(t, again[0].GetOk())

		d.setFail("10.0.0.1:2000", errors.New("dial tcp 10.0.0.1:2000: connection refused"))
		nextPass(d)
		got := h.take()
		require.Len(t, got, 1)
		require.False(t, got[0].GetOk())
		require.Equal(t, "dial tcp 10.0.0.1:2000: connection refused", got[0].GetError())

		out := scrape(t, reg)
		require.Contains(t, out, `werbot_gateway_health_checks_total{result="ok"} 2`)
		require.Contains(t, out, `werbot_gateway_health_checks_total{result="fail"} 1`)
	})
}

func TestHealthRetriesWithoutSnapshot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		d := &dialLog{}
		h, _ := healthEnv(d.dial, hres(1, "hprobe", "10.0.0.1"))
		var have atomic.Bool
		pol := h.policy()
		h.policy = func() *Policy {
			if have.Load() {
				return pol
			}
			return nil
		}
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		go func() { defer close(done); h.run(ctx, uid(0), func() []uuid.UUID { return nil }) }()
		defer func() { cancel(); <-done }()

		synctest.Wait()
		require.Empty(t, d.take())
		have.Store(true)
		time.Sleep(2 * healthRetry)
		synctest.Wait()
		require.Len(t, d.take(), 2, "the first pass follows the snapshot within seconds")
	})
}
