package tunnel

import (
	"context"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// Command approval limits: how far back Connect resends decisions, and how
// many decisions one stream queues before the hub closes it.
const (
	ApprovalResendWindow = 10 * time.Minute
	ControlQueueSize     = 64
)

// Approval is a command approval as its gateway instance sees it.
type Approval struct {
	ID        uuid.UUID
	Status    string // pending | approved | denied | expired | canceled
	ExpiresAt time.Time
	URL       string
}

// ApprovalRequest asks to allow one command of a session of instance InstanceID.
type ApprovalRequest struct {
	OrgID, InstanceID, UserID, ResourceID uuid.UUID
	AgentRunID                            *uuid.UUID
	Command                               string
	Classes, Reasons                      []string
}

// Approvals stores the command approvals of gateway instances.
type Approvals interface {
	Request(ctx context.Context, r ApprovalRequest) (Approval, error)
	Get(ctx context.Context, orgID, instanceID, id uuid.UUID) (Approval, error)                         // another instance's: ErrNotFound
	DecidedSince(ctx context.Context, orgID, instanceID uuid.UUID, since time.Time) ([]Approval, error) // approved or denied
}

// RequestApproval asks the approvers to allow a command on a resource of the caller's pool.
func (h *Handler) RequestApproval(ctx context.Context, req *connect.Request[gatewayv1.RequestApprovalRequest]) (*connect.Response[gatewayv1.RequestApprovalResponse], error) {
	g, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	r := ApprovalRequest{OrgID: g.OrgID, InstanceID: g.InstanceID, Command: m.GetCommand(), Classes: m.GetClasses(), Reasons: m.GetReasons()}
	if r.UserID, err = parseID(m.GetUserId(), "user_id"); err != nil {
		return nil, err
	}
	if r.ResourceID, err = parseID(m.GetResourceId(), "resource_id"); err != nil {
		return nil, err
	}
	if r.AgentRunID, err = optionalID(m.GetAgentRunId(), "agent_run_id"); err != nil {
		return nil, err
	}
	if _, err := h.ownResource(ctx, g, r.ResourceID); err != nil {
		return nil, err
	}
	a, err := h.d.Approvals.Request(ctx, r)
	if err != nil {
		return nil, h.fail(ctx, "request approval", err)
	}
	return connect.NewResponse(&gatewayv1.RequestApprovalResponse{ApprovalId: a.ID.String(),
		ExpiresAt: timestamppb.New(a.ExpiresAt), Url: a.URL, Status: a.Status}), nil
}

// GetApproval returns the status of a command approval of the calling instance.
func (h *Handler) GetApproval(ctx context.Context, req *connect.Request[gatewayv1.GetApprovalRequest]) (*connect.Response[gatewayv1.GetApprovalResponse], error) {
	g, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetApprovalId(), "approval_id")
	if err != nil {
		return nil, err
	}
	a, err := h.d.Approvals.Get(ctx, g.OrgID, g.InstanceID, id)
	if err != nil {
		return nil, h.fail(ctx, "get approval", err)
	}
	return connect.NewResponse(&gatewayv1.GetApprovalResponse{Status: a.Status}), nil
}

// ApprovalDecided queues the approval of payload "<instance_id>:<approval_id>"
// on that instance's streams on this replica; malformed payloads are logged
// and dropped. It never blocks the listener: a full queue closes the stream
// with "overflow", and the reconnected gateway gets the decision resent.
func (h *Hub) ApprovalDecided(payload string) {
	inst, id, err := parseDecided(payload)
	if err != nil {
		h.logger().Warn("tunnel: ignoring approval_decided payload", "payload", payload)
		return
	}
	overflow := false
	h.mu.Lock()
	if h.h != nil && h.h.d.Approvals != nil {
		// ponytail: linear scan over the replica's streams; index them by instance if NOTIFY volume grows
		for _, set := range h.streams {
			for s := range set {
				if s.gw.InstanceID != inst {
					continue
				}
				select {
				case s.ctl <- id:
				default:
					overflow = true
					s.close(unavailable("overflow"))
				}
			}
		}
	}
	h.mu.Unlock()
	if overflow {
		h.logger().Warn("tunnel: approval decision overflow", "instance_id", inst)
	}
}

func parseDecided(payload string) (inst, id uuid.UUID, err error) {
	a, b, _ := strings.Cut(payload, ":")
	if inst, err = uuid.Parse(a); err != nil {
		return
	}
	id, err = uuid.Parse(b)
	return
}

type connectStream = connect.BidiStream[gatewayv1.ConnectRequest, gatewayv1.ConnectResponse]

// resendDecided sends the decisions of the last ApprovalResendWindow, which
// a reconnected gateway may have missed; a failed read only logs.
func (h *Handler) resendDecided(ctx context.Context, st *connectStream, g Gateway) error {
	if h.d.Approvals == nil {
		return nil
	}
	as, err := h.d.Approvals.DecidedSince(ctx, g.OrgID, g.InstanceID, time.Now().Add(-ApprovalResendWindow))
	if err != nil {
		h.log.Warn("tunnel: resend approval decisions", "instance_id", g.InstanceID, "error", err)
		return nil
	}
	for _, a := range as {
		if err := sendDecided(st, a); err != nil {
			return err
		}
	}
	return nil
}

// sendDecision sends approval id of the queue once it is decided; a failed read only logs.
func (h *Handler) sendDecision(ctx context.Context, st *connectStream, g Gateway, id uuid.UUID) error {
	a, err := h.d.Approvals.Get(ctx, g.OrgID, g.InstanceID, id)
	if err != nil {
		h.log.Warn("tunnel: read approval decision", "instance_id", g.InstanceID, "approval_id", id, "error", err)
		return nil
	}
	return sendDecided(st, a)
}

// sendDecided sends a as approval_decided when it is approved or denied.
func sendDecided(st *connectStream, a Approval) error {
	if a.Status != "approved" && a.Status != "denied" {
		return nil
	}
	return st.Send(&gatewayv1.ConnectResponse{Msg: &gatewayv1.ConnectResponse_ApprovalDecided{
		ApprovalDecided: &gatewayv1.ApprovalDecided{ApprovalId: a.ID.String(), Status: a.Status}}})
}
