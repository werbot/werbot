package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// approvalPollInterval is how often a wait reads the approval status: the
// fallback for a decision the stream did not bring.
const approvalPollInterval = 15 * time.Second

// approvalRequest asks the approvers to allow one command; Reasons are the
// ids of the rules that asked for the approval, AgentRunID the run of an
// agent's command (uuid.Nil: a person's).
type approvalRequest struct {
	UserID, ResourceID, AgentRunID uuid.UUID
	Command                        string
	Classes, Reasons               []string
}

// approvalTicket is an approval: the page of the approvers, when it expires
// and its status, "approved" for an agent's command approved before.
type approvalTicket struct {
	ID, URL, Status string
	ExpiresAt       time.Time
}

// approvalOutcomes end a wait by the approval status; any other status waits on.
var approvalOutcomes = map[string]error{
	"approved": nil,
	"denied":   ErrApprovalDenied,
	"canceled": ErrApprovalDenied,
	"expired":  ErrApprovalTimeout,
}

// approvals wakes the waits for approval decisions, by approval id. The
// decision on the stream is a hint: the wait reads the status back.
type approvals struct {
	mu   sync.Mutex
	subs map[string]chan struct{}
}

// subscribe returns a channel that receives when a decision on id arrives,
// and the func that ends the subscription.
func (a *approvals) subscribe(id string) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.subs == nil {
		a.subs = map[string]chan struct{}{}
	}
	a.subs[id] = ch
	return ch, func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.subs[id] == ch {
			delete(a.subs, id)
		}
	}
}

// deliver wakes the wait for id without blocking; a decision nobody waits
// for, or one the wait has not taken yet, is dropped.
func (a *approvals) deliver(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	select {
	case a.subs[id] <- struct{}{}:
	default:
	}
}

// dispatch hands a control message of the stream to its waiter or hook.
func (c *tunnelClient) dispatch(msg *gatewayv1.ConnectResponse) {
	if d := msg.GetApprovalDecided(); d != nil {
		c.approvals.deliver(d.GetApprovalId())
	}
	if m := msg.GetMfaConfirmed(); m != nil {
		c.mfaWaits.deliver(m.GetChallengeId())
	}
	if o := msg.GetOpenStream(); o != nil {
		c.command("open_stream", o.GetTicketId(), c.onOpenStream)
	}
	if t := msg.GetTerminate(); t != nil {
		c.command("terminate", t.GetSessionId(), c.onTerminate)
	}
}

// SubscribeApproval wakes the returned channel when the stream brings a
// decision on approval id, until unsubscribe is called.
func (c *tunnelClient) SubscribeApproval(id string) (decided <-chan struct{}, unsubscribe func()) {
	return c.approvals.subscribe(id)
}

// approve asks the approvers for the command of req, which d denied with the
// approval obligation, showing them text, tells the user where to look and
// waits for the decision; an approval granted before needs no wait. An
// unreachable server is ErrNotReady; a refused request, or a denial, refuses
// the command.
func (p *Pipeline) approve(ctx context.Context, req AccessRequest, d Decision, text string, notice io.Writer) error {
	t, err := p.ctl.RequestApproval(ctx, approvalRequest{UserID: req.User.ID, ResourceID: req.Resource.ID, AgentRunID: req.User.runID(),
		Command: text, Classes: req.Classes, Reasons: d.Reasons})
	if unreachable(err) {
		return controlErr(err)
	}
	if err != nil {
		p.log.Warn("approval: request refused", "user_id", req.User.ID, "resource_id", req.Resource.ID, "error", err)
		return &CommandDeniedError{Err: ErrApprovalDenied}
	}
	if err := p.wait(ctx, t, notice); !errors.Is(err, ErrApprovalDenied) {
		return err
	}
	return &CommandDeniedError{Err: ErrApprovalDenied, Message: d.Message, Reasons: d.Reasons}
}

// wait tells the user on notice where to look, waits for the decision on t
// and, once approved, says so on notice: a client then knows the command
// runs and is no longer waiting. An approval granted before (an agent
// asking again) needs no wait and no notice.
func (p *Pipeline) wait(ctx context.Context, t approvalTicket, notice io.Writer) error {
	if t.Status == "approved" {
		return nil
	}
	if notice == nil {
		return p.awaitApproval(ctx, t)
	}
	_, _ = fmt.Fprintf(notice, "Waiting for approval %s, timeout %s\r\n%s\r\n", t.ID, formatWait(t.ExpiresAt.Sub(p.now())), t.URL)
	err := p.awaitApproval(ctx, t)
	if err == nil {
		_, _ = io.WriteString(notice, "Approved\r\n")
	}
	return err
}

// awaitApproval waits for the decision on t: a hint on the stream or every
// approvalPollInterval it reads the status; at ExpiresAt it reads it a last
// time. It returns nil once approved, ErrApprovalDenied, ErrApprovalTimeout,
// the context error when the client leaves and ErrNotReady when the gateway
// drains. A failed read is logged and the wait goes on.
func (p *Pipeline) awaitApproval(ctx context.Context, t approvalTicket) error {
	decided, unsubscribe := p.ctl.SubscribeApproval(t.ID)
	defer unsubscribe()
	poll := time.NewTicker(approvalPollInterval)
	defer poll.Stop()
	deadline := time.NewTimer(t.ExpiresAt.Sub(p.now()))
	defer deadline.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.active.drained:
			return ErrNotReady
		case <-deadline.C:
			if p.approvalStatus(ctx, t.ID) == "approved" {
				return nil
			}
			return ErrApprovalTimeout
		case <-decided:
		case <-poll.C:
		}
		if err, done := approvalOutcomes[p.approvalStatus(ctx, t.ID)]; done {
			return err
		}
	}
}

// approvalStatus reads the status of approval id; "" when the read fails.
func (p *Pipeline) approvalStatus(ctx context.Context, id string) string {
	status, err := p.ctl.GetApproval(ctx, id)
	if err != nil {
		p.log.Warn("approval: status read failed, still waiting", "approval_id", id, "error", err)
	}
	return status
}

// formatWait is d to the second: whole minutes as "5m", else seconds as "90s".
func formatWait(d time.Duration) string {
	d = d.Round(time.Second)
	if d%time.Minute == 0 {
		return fmt.Sprintf("%dm", d/time.Minute)
	}
	return fmt.Sprintf("%ds", d/time.Second)
}
