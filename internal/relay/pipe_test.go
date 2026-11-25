package relay

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

var nobody = principal.Principal{}

func TestObserveLocalBridge(t *testing.T) {
	c := newCluster(t, 1, nil)
	b := c.rs[0]
	g := startGateway(t, c, b)

	p, err := b.Observe(context.Background(), principal.Principal{}, uuid.New())
	require.NoError(t, err)
	take(t, g.ids)
	gw := peer(t, g)
	assert.EqualValues(t, 1, streams(t, b))

	require.NoError(t, gw.Write([]byte("out")))
	assert.Equal(t, "out", readData(t, p))
	require.NoError(t, gw.Resize(120, 40))
	f := nextFrame(t, p)
	assert.Equal(t, [2]int{120, 40}, [2]int{f.cols, f.rows})

	require.NoError(t, gw.Close(ReasonClosed))
	err = within(t, func() error { _, _, _, err := p.Read(); return err })
	assert.Equal(t, CloseError{Reason: ReasonClosed}, err)
	require.NoError(t, take(t, g.results))
	drained(t, b)
}

func TestAwaitTimeout(t *testing.T) {
	c := newCluster(t, 1, func(_ int, r *Relay) { r.awaitTimeout = 200 * time.Millisecond })
	b := c.rs[0]
	g := startGateway(t, c, nil)

	_, err := b.Observe(context.Background(), principal.Principal{}, uuid.New())
	require.ErrorIs(t, err, ErrAwaitTimeout)
	assert.ErrorIs(t, err, apperr.ErrUnavailable)
	assert.EqualValues(t, 1, rejected(t, b, "timeout"))
	drained(t, b)

	late := g.bridge(b, take(t, g.ids))
	assert.Equal(t, ReasonNotFound, readClose(t, late), "a bridge after the timeout finds no waiter")
	require.NoError(t, take(t, g.results))
}

func TestRunShutdownClosesStreams(t *testing.T) {
	c := newCluster(t, 1, nil)
	b := c.rs[0]
	g := startGateway(t, c, b)
	p, err := b.Observe(context.Background(), principal.Principal{}, uuid.New())
	require.NoError(t, err)
	gw := peer(t, g)

	require.NoError(t, c.stops[0]())
	assert.Equal(t, ReasonError, readClose(t, p))
	assert.Equal(t, ReasonError, readClose(t, gw))
	_, ok := lastSeen(t, c.pool, b.cfg.InstanceID)
	assert.False(t, ok, "row removed on exit")
	_, err = b.Observe(context.Background(), principal.Principal{}, uuid.New())
	assert.ErrorIs(t, err, apperr.ErrUnavailable)
}

// A gateway stream that arrives after the shutdown closed the live streams
// is closed with error, here or on the browser's replica, not handed to the
// browser still waiting for it.
func TestBridgeAfterCloseAllClosesWithError(t *testing.T) {
	c := newCluster(t, 2, nil)
	bridge := c.rs[1]
	bridge.closeAll()
	for name, target := range map[string]*Relay{"local": bridge, "remote": c.rs[0]} {
		id, _, _ := c.tk.add(target.cfg.InstanceID)
		waiter := make(chan *stream, 1)
		target.mu.Lock()
		target.waits[id] = waiter
		target.mu.Unlock()
		gw, p := memPair()
		ctx := WithCaller(context.Background(), c.tk.org, c.tk.inst)
		require.NoError(t, within(t, func() error { return bridge.Bridge(ctx, id, gw) }), name)
		assert.Equal(t, ReasonError, readClose(t, p), name)
		assert.Empty(t, waiter, name)
	}
	drained(t, c.rs...)
}

func TestStreamLimit(t *testing.T) {
	c := newCluster(t, 2, func(i int, r *Relay) { r.maxStreams = []int{2, 0}[i] })
	b, bridge := c.rs[0], c.rs[1]
	startGateway(t, c, b)
	p1, err := b.Observe(context.Background(), nobody, uuid.New())
	require.NoError(t, err)
	_, err = b.Observe(context.Background(), nobody, uuid.New())
	require.NoError(t, err)
	_, err = b.Terminal(context.Background(), nobody, "web1", 80, 24)
	require.ErrorIs(t, err, ErrLimit)
	assert.EqualValues(t, 1, rejected(t, b, "limit"))

	require.NoError(t, p1.Close(ReasonClosed))
	require.Eventually(t, func() bool { return streams(t, b) == 1 }, 5*time.Second, 10*time.Millisecond)
	_, err = b.Terminal(context.Background(), nobody, "web1", 80, 24)
	require.NoError(t, err, "a released slot is free again")

	id, _, _ := c.tk.add(b.cfg.InstanceID)
	gw, _ := memPair()
	err = bridge.Bridge(WithCaller(context.Background(), c.tk.org, c.tk.inst), id, gw)
	require.ErrorIs(t, err, apperr.ErrUnavailable, "a remote bridge needs a slot")
	assert.EqualValues(t, 1, rejected(t, bridge, "limit"))
}
