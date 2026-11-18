package tunnel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync/atomic"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// refreshOrg drops streams whose instance no longer authenticates, then reads
// the organization-wide part once, completes it per pool (up to poolParallel
// pools at once) and offers each snapshot to that pool's streams. It reports
// false when a build failed.
func (hd *Handler) refreshOrg(ctx context.Context, org uuid.UUID, ss []*stream, gen uint64) bool {
	pools := map[uuid.UUID][]*stream{}
	for _, s := range ss {
		if _, err := hd.d.Gateways.Authenticate(ctx, s.gw.ID, s.gw.InstanceID, s.serial); err != nil {
			hd.log.Info("tunnel: closing stream of a rejected gateway", "instance_id", s.gw.InstanceID, "error", err)
			s.close(authError(err))
			continue
		}
		pools[s.gw.ID] = append(pools[s.gw.ID], s)
	}
	if len(pools) == 0 {
		return true
	}
	base, err := hd.d.Snapshots.Org(ctx, org)
	if err != nil {
		hd.log.Error("tunnel: build snapshot, retrying", "org_id", org, "error", err)
		return false
	}
	var failed atomic.Bool
	var g errgroup.Group
	g.SetLimit(poolParallel)
	for pool, ps := range pools {
		g.Go(func() error {
			snap, err := hd.poolSnapshot(ctx, base, org, pool)
			if err != nil {
				hd.log.Error("tunnel: build snapshot, retrying", "org_id", org, "gateway_id", pool, "error", err)
				failed.Store(true)
				return nil
			}
			for _, s := range ps {
				s.offer(snap, gen)
			}
			return nil
		})
	}
	_ = g.Wait() // every pool reports through failed
	return !failed.Load()
}

// build returns the pool snapshot stamped with its version.
func (hd *Handler) build(ctx context.Context, org, pool uuid.UUID) (*gatewayv1.Snapshot, error) {
	base, err := hd.d.Snapshots.Org(ctx, org)
	if err != nil {
		return nil, err
	}
	return hd.poolSnapshot(ctx, base, org, pool)
}

// poolSnapshot completes the organization part base for pool and stamps the version.
func (hd *Handler) poolSnapshot(ctx context.Context, base *gatewayv1.Snapshot, org, pool uuid.UUID) (*gatewayv1.Snapshot, error) {
	snap, err := hd.d.Snapshots.Pool(ctx, base, org, pool)
	if err != nil {
		return nil, err
	}
	snap.Version = snapshotVersion(snap)
	return snap, nil
}

// snapshotVersion is the hex SHA-256 of the deterministic encoding of s without its version.
func snapshotVersion(s *gatewayv1.Snapshot) string {
	c := proto.CloneOf(s)
	c.Version = ""
	// A marshal error (invalid UTF-8) also fails the Send, so the hash is never used.
	b, _ := proto.MarshalOptions{Deterministic: true}.Marshal(c)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
