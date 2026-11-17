package gateway

import (
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/gateway/recorder"
	"github.com/werbot/werbot/internal/platform/version"
)

const (
	countersEvery = 4    // every 4th heartbeat (60 s) carries counters
	maxCounters   = 5000 // max_items of Heartbeat.counters; the rest waits for the next one
)

// heartbeat is the instance state sent on the tunnel: the session ids, the
// refused connections and changed health results since the last one and, every countersEvery-th time, the
// counters that changed since they were last sent.
func (p *Pipeline) heartbeat() *gatewayv1.Heartbeat {
	hb := &gatewayv1.Heartbeat{
		Version: version.Version, Connectors: connectors(), ActiveSessionIds: heartbeatSessionIDs(p.active, p.spool),
		Denials: p.denials.take(), Health: p.health.take(),
	}
	if p.beats.Add(1)%countersEvery == 0 {
		hb.Counters = p.active.changedCounters(maxCounters)
	}
	return hb
}

// changedCounters lists up to limit sessions whose counters changed since the
// last call and marks them sent.
func (a *activeSessions) changedCounters(limit int) []*gatewayv1.SessionCounters {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []*gatewayv1.SessionCounters
	for id, t := range a.sessions {
		if len(out) == limit {
			break
		}
		if v := t.c.load(); v != t.sent {
			t.sent = v
			out = append(out, countersProto(id, v))
		}
	}
	return out
}

// heartbeatSessionIDs lists the opened sessions, then those whose end waits in the
// spool, without repeats; the server closes any other session of this instance as
// lost. Opened sessions are read first: a session moves from them to the spool, so
// it cannot slip between the two reads. Ids beyond maxSessionIDs (pending ends only,
// as MaxSessions is capped by it) wait for a later heartbeat.
func heartbeatSessionIDs(a *activeSessions, sp *recorder.Spool) []string {
	ids := a.ids()
	seen := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		seen[id] = true
	}
	for _, id := range sp.PendingEnds() {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	a.noteOmitted(max(len(ids)-maxSessionIDs, 0))
	out := make([]string, min(len(ids), maxSessionIDs))
	for i := range out {
		out[i] = ids[i].String()
	}
	return out
}
