package tunnel

import (
	"context"
	"sync"

	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// newBuilds shares the snapshot builds of new streams per (org, pool). A
// build serves only streams that asked before it started, so no stream gets
// data older than a build of its own would read. A stream that asks while a
// build of its pool runs waits for the next one, which starts when the
// running one ends and serves every stream that asked meanwhile.
type newBuilds struct {
	mu    sync.Mutex
	pools map[[2]uuid.UUID]*poolBuilds // present while a build of the pool runs
}

type poolBuilds struct {
	next *buildResult // the build that starts after the running one; nil: none asked
}

type buildResult struct {
	done   chan struct{} // closed when snap and err are set
	snap   *gatewayv1.Snapshot
	err    error
	joined int // streams waiting for it
}

// newSnapshot returns a pool snapshot read after this call began, for the
// stream of a new Connect. The build runs without the caller's cancel, as
// other streams may wait for it; the caller stops waiting when ctx ends.
func (hd *Handler) newSnapshot(ctx context.Context, org, pool uuid.UUID) (*gatewayv1.Snapshot, error) {
	b := &hd.newBuilds
	key := [2]uuid.UUID{org, pool}
	b.mu.Lock()
	if b.pools == nil {
		b.pools = map[[2]uuid.UUID]*poolBuilds{}
	}
	p, running := b.pools[key]
	if !running {
		p = &poolBuilds{}
		b.pools[key] = p
	}
	if p.next == nil {
		p.next = &buildResult{done: make(chan struct{})}
	}
	r := p.next
	r.joined++
	if !running {
		p.next = nil
		bctx := context.WithoutCancel(ctx)
		go b.run(key, p, r, func() (*gatewayv1.Snapshot, error) { return hd.build(bctx, org, pool) })
	}
	b.mu.Unlock()
	select {
	case <-r.done:
		return r.snap, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// run builds r, then each next build streams asked for meanwhile, until none did.
func (b *newBuilds) run(key [2]uuid.UUID, p *poolBuilds, r *buildResult, build func() (*gatewayv1.Snapshot, error)) {
	for r != nil {
		r.snap, r.err = build()
		close(r.done)
		b.mu.Lock()
		r, p.next = p.next, nil
		if r == nil {
			delete(b.pools, key)
		}
		b.mu.Unlock()
	}
}
