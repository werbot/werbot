package gateway

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// A check that ends after the heartbeat took the probe result rides the next
// heartbeat with the latest probe result of the resource: pending turns ok
// within one pass. The result carries the time the check ended.
func TestReadonlyResultWithoutPendingHealth(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		useRO(t, func(context.Context) (string, string) { <-release; return ReadonlyOK, "" })
		e := newROEnv(rores(1, "pending", 1))
		e.h.pass(t.Context(), uid(0), nil)
		first := e.h.take()
		require.Len(t, first, 1)
		status, _, _ := readonlyOf(first)
		require.Empty(t, status, "the check has not ended")

		time.Sleep(time.Second)
		close(release)
		synctest.Wait()
		got := e.h.take()
		require.Len(t, got, 1, "the result does not wait for the next pass")
		status, _, version := readonlyOf(got)
		require.Equal(t, ReadonlyOK, status)
		require.EqualValues(t, 5, version)
		require.True(t, got[0].GetOk())
		require.True(t, got[0].GetCheckedAt().AsTime().Equal(first[0].GetCheckedAt().AsTime()), "the latest probe result goes again")
		require.True(t, got[0].GetAgentReadonlyCheckedAt().AsTime().Equal(time.Now()), "the time the check ended")
		require.Empty(t, e.h.take())
	})
}

// A heartbeat that did not reach the server gives its check results back: the
// next heartbeat carries them, unless a newer check of the resource replaced
// them meanwhile. Probe results are not given back: every pass sends them.
func TestReadonlyResultKeptOnFailedHeartbeat(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		useRO(t, nil)
		e := newROEnv(rores(1, "pending", 1))
		e.pass(t.Context())
		lost := e.h.take()
		e.h.requeue(lost)
		got := e.h.take()
		require.Len(t, got, 1)
		status, _, _ := readonlyOf(got)
		require.Equal(t, ReadonlyOK, status, "the undelivered result goes again")

		e.h.requeue(got)
		useRO(t, func(context.Context) (string, string) { return ReadonlyHasWrite, "INSERT on db.public.t" })
		time.Sleep(time.Second)
		e.creds.set(func(c *probeCreds) { c.version = 6 })
		e.pass(t.Context())
		e.h.requeue(lost)
		status, _, version := readonlyOf(e.h.take())
		require.Equal(t, [2]any{ReadonlyHasWrite, int64(6)}, [2]any{status, version}, "an older result never replaces a newer one")
	})
}

// A version whose result still waits for a heartbeat is checked again in the
// next pass instead of being skipped for readonlyRecheck.
func TestReadonlyRechecksUndeliveredVersion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := useRO(t, nil)
		e := newROEnv(rores(1, "pending", 1))
		e.pass(t.Context())
		e.pass(t.Context())
		require.Len(t, f.calls(), 2, "the result was never delivered")
		e.h.take()
		e.pass(t.Context())
		require.Len(t, f.calls(), 2, "a delivered version is not checked again")
	})
}

// The checks keep only the resources this instance still probes: a deleted
// resource or one that moved to other probers leaves no entry behind.
func TestReadonlyChecksPruned(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		useRO(t, nil)
		e := newROEnv(rores(1, "pending", 1), rores(2, "pending", 1), rores(3, "pending", 1))
		e.pass(t.Context())
		require.Equal(t, 3, e.h.readonly.entries())

		e.setResources(rores(1, "pending", 1), rores(2, "pending", 1))
		e.pass(t.Context())
		require.Equal(t, 2, e.h.readonly.entries(), "the deleted resource is dropped")

		var pool []uuid.UUID
		for i := 10; i < 20; i++ {
			pool = append(pool, uid(i))
		}
		outsider := slices.IndexFunc(pool, func(id uuid.UUID) bool {
			return !slices.Contains(probers(uid(1), pool, probersPerResource), id) &&
				!slices.Contains(probers(uid(2), pool, probersPerResource), id)
		})
		require.GreaterOrEqual(t, outsider, 0)
		e.h.pass(t.Context(), pool[outsider], pool)
		synctest.Wait()
		require.Zero(t, e.h.readonly.entries(), "resources of other probers are dropped")
	})
}

// entries counts the resources the checks keep any state of.
func (c *readonlyChecks) entries() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	ids := map[uuid.UUID]bool{}
	for id := range c.last {
		ids[id] = true
	}
	for id := range c.results {
		ids[id] = true
	}
	return len(ids)
}

// A heartbeat whose send fails is handed to undelivered and never reached
// the server; the stream sends the next one.
func TestTunnelUndeliveredHeartbeat(t *testing.T) {
	e := newTunnelEnv(t, func(int) attempt {
		return attempt{snap: testSnap("v1"), closeWith: connect.NewError(connect.CodeUnavailable, errors.New("rebalance")),
			closeAfter: 20 * time.Millisecond}
	}, time.Hour)
	e.c.heartbeatEvery = time.Millisecond
	var mu sync.Mutex
	var seq int
	lost := map[string]bool{}
	heartbeat := func() *gatewayv1.Heartbeat {
		mu.Lock()
		defer mu.Unlock()
		seq++
		return &gatewayv1.Heartbeat{Version: strconv.Itoa(seq)}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- e.c.Run(ctx, heartbeat, func(hb *gatewayv1.Heartbeat) {
			mu.Lock()
			defer mu.Unlock()
			lost[hb.GetVersion()] = true
		})
	}()
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(lost) > 0
	}, 10*time.Second, time.Millisecond)
	cancel()
	require.NoError(t, <-done)
	e.f.mu.Lock()
	defer e.f.mu.Unlock()
	mu.Lock()
	defer mu.Unlock()
	for _, hb := range e.f.hbs {
		require.False(t, lost[hb.GetVersion()], "a heartbeat the server got was reported undelivered")
	}
}
