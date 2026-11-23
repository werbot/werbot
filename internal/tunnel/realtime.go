package tunnel

import (
	"context"
	"net/netip"
	"strings"

	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// WebTicket is a stream ticket of the calling instance: Mode "web" opens a
// terminal on Resource, "observe" watches SessionID; ClientIP is the browser's.
type WebTicket struct {
	Mode, Resource string
	SessionID      *uuid.UUID
	Cols, Rows     int
	ClientIP       netip.Addr
}

// ctlKind is what a control message asks a Connect stream to send.
type ctlKind uint8

const (
	approvalDecided    ctlKind = iota // the decision of approval id
	openStream                        // open_stream for ticket id
	terminate                         // terminate for session id
	resendTerminations                // every pending terminate of the instance
)

// ctlMsg is one entry of a stream's control queue.
type ctlMsg struct {
	kind ctlKind
	id   uuid.UUID
}

// StreamOpen queues open_stream for payload "<instance_id>:<ticket_id>" on
// that instance's streams on this replica. A full queue drops it: the
// browser's wait times out.
func (h *Hub) StreamOpen(payload string) {
	inst, id, ok := h.parsePayload("stream_open", payload)
	if ok && h.enqueue(inst, ctlMsg{kind: openStream, id: id}, false) {
		h.logger().Warn("tunnel: stream open dropped", "instance_id", inst)
	}
}

// SessionTerminate queues terminate for payload "<instance_id>:<session_id>"
// on that instance's streams on this replica. A full queue closes the
// stream: the reconnected gateway gets the terminate resent.
func (h *Hub) SessionTerminate(payload string) {
	inst, id, ok := h.parsePayload("session_terminate", payload)
	if ok && h.enqueue(inst, ctlMsg{kind: terminate, id: id}, true) {
		h.logger().Warn("tunnel: terminate overflow", "instance_id", inst)
	}
}

// parsePayload splits "<instance_id>:<id>"; a malformed payload is logged by
// its length and the instance id, if that parsed, never by its content.
func (h *Hub) parsePayload(channel, payload string) (inst, id uuid.UUID, ok bool) {
	a, b, _ := strings.Cut(payload, ":")
	inst, err := uuid.Parse(a)
	if err != nil {
		h.logger().Warn("tunnel: ignoring "+channel+" payload", "payload_len", len(payload))
		return uuid.Nil, uuid.Nil, false
	}
	if id, err = uuid.Parse(b); err != nil {
		h.logger().Warn("tunnel: ignoring "+channel+" payload", "payload_len", len(payload), "instance_id", inst)
		return uuid.Nil, uuid.Nil, false
	}
	return inst, id, true
}

// enqueue puts m on the queue of every stream of instance inst (uuid.Nil:
// every stream) without blocking and reports whether a queue was full; a
// full queue closes its stream with "overflow" when overflowCloses.
func (h *Hub) enqueue(inst uuid.UUID, m ctlMsg, overflowCloses bool) (overflow bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	// ponytail: linear scan over the replica's streams; index them by instance if NOTIFY volume grows
	for _, set := range h.streams {
		for s := range set {
			if inst != uuid.Nil && s.gw.InstanceID != inst {
				continue
			}
			select {
			case s.ctl <- m:
			default:
				overflow = true
				if overflowCloses {
					s.close(unavailable("overflow"))
				}
			}
		}
	}
	return overflow
}

// sendControl sends what control message m asks for on the stream of g.
func (hd *Handler) sendControl(ctx context.Context, st *connectStream, g Gateway, m ctlMsg) error {
	switch m.kind {
	case approvalDecided:
		return hd.sendDecision(ctx, st, g, m.id)
	case openStream:
		return st.Send(&gatewayv1.ConnectResponse{Msg: &gatewayv1.ConnectResponse_OpenStream{
			OpenStream: &gatewayv1.OpenStream{TicketId: m.id.String()}}})
	case terminate:
		return sendTerminate(st, m.id)
	default:
		return hd.resendTerminations(ctx, st, g)
	}
}

// resend sends what a (re)connected gateway may have missed: approval
// decisions and pending terminations.
func (hd *Handler) resend(ctx context.Context, st *connectStream, g Gateway) error {
	if err := hd.resendDecided(ctx, st, g); err != nil {
		return err
	}
	return hd.resendTerminations(ctx, st, g)
}

// resendTerminations sends terminate for every open session of g asked to
// end; a failed read only logs, the stream lives.
func (hd *Handler) resendTerminations(ctx context.Context, st *connectStream, g Gateway) error {
	ids, err := hd.d.Sessions.PendingTerminations(ctx, g.OrgID, g.InstanceID)
	if err != nil {
		hd.log.Warn("tunnel: resend terminations", "instance_id", g.InstanceID, "error", err)
		return nil
	}
	for _, id := range ids {
		if err := sendTerminate(st, id); err != nil {
			return err
		}
	}
	return nil
}

// sendTerminate asks the gateway to end session id; the requester's reason
// stays in the audit log.
func sendTerminate(st *connectStream, id uuid.UUID) error {
	return st.Send(&gatewayv1.ConnectResponse{Msg: &gatewayv1.ConnectResponse_Terminate{
		Terminate: &gatewayv1.Terminate{SessionId: id.String(), Reason: "terminated"}}})
}

// userByWebTicket resolves ticket of the caller's instance; the ticket is
// read, never spent.
func (hd *Handler) userByWebTicket(ctx context.Context, g Gateway, ticket string) (User, *WebTicket, error) {
	id, err := parseID(ticket, "web_ticket")
	if err != nil {
		return User{}, nil, err
	}
	u, wt, err := hd.d.Users.ByWebTicket(ctx, g.OrgID, g.InstanceID, id)
	if err != nil {
		return User{}, nil, err
	}
	return u, &wt, nil
}

// webTicketOf is wt as the gateway gets it; nil for nil.
func webTicketOf(wt *WebTicket) *gatewayv1.WebTicket {
	if wt == nil {
		return nil
	}
	out := &gatewayv1.WebTicket{Mode: wt.Mode, Resource: wt.Resource, Cols: int32(wt.Cols), Rows: int32(wt.Rows)} //nolint:gosec // ticket windows are bounded
	if wt.SessionID != nil {
		out.SessionId = wt.SessionID.String()
	}
	if wt.ClientIP.IsValid() {
		out.ClientIp = wt.ClientIP.String()
	}
	return out
}
