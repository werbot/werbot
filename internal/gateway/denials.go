package gateway

import (
	"cmp"
	"context"
	"errors"
	"net/netip"
	"sync"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/metrics"
)

// maxDenials is the max_items of Heartbeat.denials.
const maxDenials = 100

// denials buffers refused connections until the next heartbeat; past maxDenials
// they are only counted.
// ponytail: kept in memory (lost on kill -9, at most one heartbeat apart); move to
// the spool if losing them matters.
type denials struct {
	mu      sync.Mutex
	buf     []*gatewayv1.Denial
	dropped *metrics.Counter
}

func newDenials(reg *metrics.Registry) *denials {
	return &denials{dropped: reg.Counter("werbot_gateway_denials_dropped_total", "Refused connections not reported: more than 100 per heartbeat.")}
}

func (d *denials) add(x *gatewayv1.Denial) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.buf) >= maxDenials {
		d.dropped.Inc()
		return
	}
	d.buf = append(d.buf, x)
}

// take empties the buffer for a heartbeat.
func (d *denials) take() []*gatewayv1.Denial {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := d.buf
	d.buf = nil
	return out
}

// denialReason is the Denial.reason of an Open error; "" for ErrNotReady, which is
// not reported: the server is out of reach then, and for ErrHandled: no refusal.
func denialReason(err error) string {
	switch {
	case errors.Is(err, ErrNotReady), errors.Is(err, ErrHandled):
		return ""
	case errors.Is(err, ErrBusy):
		return "busy"
	case errors.Is(err, ErrSpoolMax):
		return "spool_full"
	case errors.Is(err, ErrTargetNotAllowed):
		return "target_not_allowed"
	case errors.Is(err, ErrUnsupportedType):
		return "unsupported_type"
	case errors.Is(err, ErrUnsupportedKind):
		return "unsupported_kind"
	case errors.Is(err, ErrApprovalDenied):
		return "approval_denied"
	case errors.Is(err, ErrApprovalTimeout):
		return "approval_timeout"
	}
	return "denied"
}

// newDenial builds a Denial that passes the wire bounds: every string is cut to
// its max_bytes, an IP goes without zone and a nil user is left out. One invalid
// field would close the tunnel stream.
func newDenial(reason string, user uuid.UUID, fingerprint, resource string, kind Kind, target string, ip netip.Addr) *gatewayv1.Denial {
	d := &gatewayv1.Denial{
		KeyFingerprint: TruncateUTF8(fingerprint, 128),
		Resource:       TruncateUTF8(resource, 63),
		Kind:           TruncateUTF8(string(kind), 16),
		Target:         TruncateUTF8(target, 1024),
		Reason:         reason,
		At:             timestamppb.Now(),
	}
	if user != uuid.Nil {
		d.UserId = user.String()
	}
	if ip.IsValid() {
		d.ClientIp = ip.Unmap().WithZone("").String()
	}
	return d
}

// deny reports a refused Open of req: an agent's policy denial at once
// (reportAgentDenial), anything else in the next heartbeat; the target of a
// request with a command is the command.
func (p *Pipeline) deny(ctx context.Context, r OpenRequest, req AccessRequest, err error) {
	if !p.reportAgentDenial(ctx, r, req, err) {
		p.denyLater(r, req.User.ID, err)
	}
}

// denyLater reports a refused Open of user in the next heartbeat.
func (p *Pipeline) denyLater(r OpenRequest, user uuid.UUID, err error) {
	if reason := denialReason(err); reason != "" {
		p.denials.add(newDenial(reason, user, r.Identity.fingerprint(), r.Resource, r.Kind, cmp.Or(r.Command, r.Target), r.ClientIP))
	}
}

// fingerprint is the key fingerprint a denial of id reports; a web identity's
// value is its stream ticket, which never leaves the gateway in a denial.
func (id Identity) fingerprint() string {
	if id.Method == "web-ticket" {
		return ""
	}
	return id.Value
}
