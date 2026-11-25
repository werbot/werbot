package gateway

import (
	"slices"
	"time"
)

// StaleAfter is how long the tunnel of a strict organization may be down:
// past it the gateway ends every session with policy_stale.
const StaleAfter = 60 * time.Second

const causeStale endCause = "policy_stale"

// Expirations are the unix seconds at which the grants and locks of the
// snapshot expire, ascending.
func (p *Policy) Expirations() []int64 {
	if p == nil {
		return nil
	}
	return slices.Clone(p.expirations)
}

// LockMode is "strict" or "best_effort" (also when the server sends none).
func (p *Policy) LockMode() string {
	if p == nil || p.lockMode == "" {
		return "best_effort"
	}
	return p.lockMode
}

// nextExpiry is the first of the ascending exps after the second of now; a
// timer that fired early (now.Unix() < exp) gets the same exp back.
func nextExpiry(exps []int64, now time.Time) (time.Time, bool) {
	for _, e := range exps {
		if e > now.Unix() {
			return time.Unix(e, 0), true
		}
	}
	return time.Time{}, false
}

// armExpiry sets t to the next expiration of the current policy after now, or stops it.
func (p *Pipeline) armExpiry(t *time.Timer, now time.Time) {
	if at, ok := nextExpiry(p.ctl.Policy().Expirations(), now); ok {
		t.Reset(at.Sub(now))
		return
	}
	t.Stop()
}

// checkStale ends every session with policy_stale when the organization is
// strict and the tunnel has been down for more than StaleAfter.
func (p *Pipeline) checkStale(now time.Time) {
	since := p.ctl.DisconnectedSince()
	if p.ctl.Policy().LockMode() != "strict" || since.IsZero() || now.Sub(since) <= StaleAfter {
		return
	}
	if n := p.active.TerminateAll(string(causeStale)); n > 0 {
		p.log.Warn("policy is stale, sessions terminated", "disconnected_since", since, "sessions", n)
	}
}

// TerminateAll ends every opened session with reason (before bind the reason
// waits for it) and returns how many there are.
func (a *activeSessions) TerminateAll(reason string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	for id := range a.sessions {
		a.cancelLocked(id, endCause(reason))
	}
	return len(a.sessions)
}
