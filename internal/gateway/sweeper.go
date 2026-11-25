package gateway

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const (
	sweepEvery  = 5 * time.Second  // idle timeout and max duration checks
	reevalEvery = 60 * time.Second // re-evaluation of every session (schedules)
)

// End reasons set by the sweeper.
const (
	causeIdle    endCause = "idle_timeout"
	causeMax     endCause = "max_duration"
	causeRevoked endCause = "access_revoked"
)

// sweep ends sessions past their idle timeout or max duration, and every session
// of a strict organization whose policy is stale, every sweepEvery. It
// re-evaluates access: every session on a new snapshot, at the next grant or lock
// expiration and every reevalEvery, in between those still allowed on an older
// snapshot (opened while one arrived). It runs until ctx ends.
func (p *Pipeline) sweep(ctx context.Context) {
	tick := time.NewTicker(sweepEvery)
	defer tick.Stop()
	expiry := time.NewTimer(0)
	defer expiry.Stop()
	p.armExpiry(expiry, time.Now())
	last := time.Now()
	reeval := func() {
		now := time.Now()
		p.reevaluate(now, true)
		p.armExpiry(expiry, now)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.ctl.PolicyChanged():
			reeval()
		case <-expiry.C:
			reeval()
		case <-tick.C:
			now := time.Now()
			p.active.expire(now)
			p.checkStale(now)
			all := now.Sub(last) >= reevalEvery
			if all {
				last = now
			}
			p.reevaluate(now, all)
		}
	}
}

// expire cancels the sessions past their max duration or idle timeout.
func (a *activeSessions) expire(now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for id, t := range a.sessions {
		switch {
		case t.max > 0 && now.Sub(t.started) >= t.max:
			a.cancelLocked(id, causeMax)
		case t.idledOut(now):
			a.cancelLocked(id, causeIdle)
		}
	}
}

// idledOut reports whether t has passed its idle limit at now; a held session has not.
func (t *tracked) idledOut(now time.Time) bool {
	return t.idle > 0 && t.held == 0 && t.c.idleFor(now) >= t.idle
}

// evaluation is one session to re-evaluate, copied out of activeSessions so the
// PDP runs without its lock.
type evaluation struct {
	id   uuid.UUID
	req  AccessRequest
	kind Kind
	host string
}

// reevaluate asks the current policy again for every session (all) or for those
// allowed on an older snapshot; a refused one ends with access_revoked.
func (p *Pipeline) reevaluate(now time.Time, all bool) {
	pol := p.ctl.Policy()
	if pol == nil {
		return
	}
	for _, e := range p.active.toEvaluate(pol.Version(), all) {
		req, ok := recheck(pol, e, now)
		p.active.reevaluated(e.id, pol.Version(), req, ok, p.cfg)
	}
}

func (a *activeSessions) toEvaluate(version string, all bool) []evaluation {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []evaluation
	for id, t := range a.sessions {
		if all || t.version != version {
			out = append(out, evaluation{id: id, req: t.req, kind: t.kind, host: t.host})
		}
	}
	return out
}

// reevaluated stores an allowed request with the resource's limits, or ends the session.
func (a *activeSessions) reevaluated(id uuid.UUID, version string, req AccessRequest, ok bool, cfg Config) {
	a.mu.Lock()
	defer a.mu.Unlock()
	t := a.sessions[id]
	switch {
	case t == nil: // ended meanwhile
	case !ok:
		a.cancelLocked(id, causeRevoked)
	default:
		t.req, t.version = req, version
		t.idle, t.max = limits(req.Resource, cfg)
	}
}

// recheck is the Open decision against pol: the resource by id (a rename keeps a
// terminal or exec session; a stream or datagram session opened by the old name
// fails the target check and ends with access_revoked), the membership and teams
// of the user (when the snapshot lists them), the stream or datagram target and
// the PDP at now, for the connection and then for the command of the session.
func recheck(pol *Policy, e evaluation, now time.Time) (AccessRequest, bool) {
	req := e.req
	res, ok := pol.resourceByID(req.Resource.ID)
	if !ok {
		return req, false
	}
	req.Resource, req.Time = res, now
	if req.User, ok = pol.withTeams(req.User); !ok {
		return req, false
	}
	if !targetAllowed(res, e) {
		return req, false
	}
	allow, _ := pol.Authorize(connectOf(req))
	return req, allow && commandAllowed(pol, req, e.kind) && forwardAllowed(pol, req)
}

// limits are the idle timeout and max duration of a session on r.
func limits(r Resource, cfg Config) (idle, maxDur time.Duration) {
	idle, maxDur = r.IdleTimeout, r.MaxDuration
	if idle == 0 {
		idle = cfg.IdleTimeout
	}
	if maxDur == 0 {
		maxDur = cfg.MaxDuration
	}
	return idle, maxDur
}
