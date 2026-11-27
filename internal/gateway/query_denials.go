package gateway

import (
	"context"
	"sync"
	"time"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// Agent operation denials wait in a queue of agentDenialQueue for one
// goroutine of the gateway that reports each through RecordDenial within
// agentDenialTimeout.
const (
	agentDenialQueue   = 256
	agentDenialTimeout = 10 * time.Second
)

// queryDenial is a denied operation of session s.
type queryDenial struct {
	s  *Session
	in ApprovalInput
}

// queryDenials is the queue of agent denials; the first denial starts its goroutine.
type queryDenials struct {
	once sync.Once
	ch   chan queryDenial
}

// recordDenial is the RecordDenial hook of s. A person's denial, and an
// agent's denial by its run's state, go to the next heartbeat; any other
// agent denial to RecordDenial through the queue, never blocking the caller:
// past the queue, or when the call fails, it goes to the next heartbeat with
// the run.
func (p *Pipeline) recordDenial(s *Session) func(ApprovalInput) {
	return func(in ApprovalInput) {
		if s.User.Agent == nil || stateDenial(in.Reasons) {
			p.denials.add(queryDenialOf(s, in))
			return
		}
		q := &p.queryDenials
		q.once.Do(func() {
			q.ch = make(chan queryDenial, agentDenialQueue)
			go p.drainQueryDenials(q.ch)
		})
		select {
		case q.ch <- queryDenial{s: s, in: in}:
		default:
			p.denials.add(queryDenialOf(s, in))
		}
	}
}

// drainQueryDenials reports the queued agent denials until the pipeline stops.
func (p *Pipeline) drainQueryDenials(ch <-chan queryDenial) {
	for {
		select {
		case <-p.stop.Done():
			return
		case d := <-ch:
			p.sendQueryDenial(d)
		}
	}
}

func (p *Pipeline) sendQueryDenial(d queryDenial) {
	ctx, cancel := context.WithTimeout(p.stop, agentDenialTimeout)
	defer cancel()
	x := queryDenialOf(d.s, d.in)
	if _, err := p.ctl.RecordDenial(ctx, d.s.User.Agent.RunID, x.GetResource(), x.GetTarget(), x.GetClasses(), x.GetReasons()); err != nil {
		p.log.Warn("agent: query denial not recorded, reported in the next heartbeat", "run_id", d.s.User.Agent.RunID, "error", err)
		p.denials.add(x)
	}
}

// queryDenialOf is the in-session denial of an operation of s: its text
// without the session credential, cut to the wire bounds.
func queryDenialOf(s *Session, in ApprovalInput) *gatewayv1.Denial {
	d := newDenial("denied", s.User.ID, "", s.Resource.Name, s.Kind, s.Secret.Redact(in.Command), s.ClientIP)
	d.SessionId, d.Classes = s.ID.String(), in.Classes
	d.Reasons = denialReasons(in.Reasons)
	if a := s.User.Agent; a != nil {
		d.AgentRunId = a.RunID.String()
	}
	return d
}
