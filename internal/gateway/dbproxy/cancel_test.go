package dbproxy_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway/dbproxy"
)

// countTarget counts its calls; order records "on" (OnCancel) and "cancel" (Cancel).
type countTarget struct {
	mu    sync.Mutex
	order []string
}

func (c *countTarget) target(resourceID uuid.UUID) dbproxy.CancelTarget {
	add := func(s string) { c.mu.Lock(); c.order = append(c.order, s); c.mu.Unlock() }
	return dbproxy.CancelTarget{
		ResourceID: resourceID,
		OnCancel:   func() { add("on") },
		Cancel:     func(context.Context) error { add("cancel"); return nil },
	}
}

func (c *countTarget) calls() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.order...)
}

func TestCancelRegistry(t *testing.T) {
	var r dbproxy.CancelRegistry // the zero value is ready
	ctx, rid := context.Background(), uuid.New()

	keys := map[uint64]bool{}
	for range 1000 {
		k, _ := r.Register(dbproxy.CancelTarget{ResourceID: rid})
		require.NotZero(t, k)
		require.False(t, keys[k], "keys must be unique")
		keys[k] = true
	}

	ct := &countTarget{}
	key, release := r.Register(ct.target(rid))
	assert.True(t, r.Cancel(ctx, key, rid))
	assert.Equal(t, []string{"on", "cancel"}, ct.calls(), "OnCancel then Cancel, once each")

	assert.False(t, r.Cancel(ctx, key^1, rid), "an unknown key cancels nothing")
	release()
	release() // repeated release is a no-op
	assert.False(t, r.Cancel(ctx, key, rid), "a released key cancels nothing")
	assert.Len(t, ct.calls(), 2)
}

func TestCancelRegistryOtherResource(t *testing.T) {
	var r dbproxy.CancelRegistry
	ct := &countTarget{}
	key, release := r.Register(ct.target(uuid.New()))
	defer release()
	assert.False(t, r.Cancel(context.Background(), key, uuid.New()), "a key of another resource cancels nothing")
	assert.False(t, r.Cancel(context.Background(), key, uuid.Nil))
	assert.Empty(t, ct.calls())
}

func TestCancelRegistryConcurrent(t *testing.T) {
	var r dbproxy.CancelRegistry
	rid := uuid.New()
	var cancels atomic.Int64
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			for range 100 {
				key, release := r.Register(dbproxy.CancelTarget{
					ResourceID: rid,
					OnCancel:   func() {},
					Cancel:     func(context.Context) error { cancels.Add(1); return nil },
				})
				r.Cancel(context.Background(), key, rid)
				release()
			}
		})
	}
	wg.Wait()
	assert.Equal(t, int64(1600), cancels.Load())
}
