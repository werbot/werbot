package gateway

import (
	"context"
	"slices"
)

// decide authenticates the user of r and asks the PDP whether they may open
// it: the resource must exist and, for a stream or datagram, the target must be
// one of its endpoints. The request it decided comes back with an error too, so
// the denial names the user once known.
func (p *Pipeline) decide(ctx context.Context, r OpenRequest) (pol *Policy, req AccessRequest, host string, err error) {
	if pol = p.ctl.Policy(); pol == nil || !p.ctl.Connected() {
		return nil, req, "", ErrNotReady
	}
	var reused, ok bool
	if req.User, reused, err = p.user(ctx, r); err != nil {
		return nil, AccessRequest{}, "", controlErr(err)
	}
	req.IP, req.Kind = r.ClientIP, r.Kind
	if r.Identity.Cert != nil {
		req.CLISessionID = r.Identity.Cert.CLISessionID
	}
	// The handshake's teams may predate pol, which stamps the session: take pol's.
	if reused {
		if req.User, ok = pol.withTeams(req.User); !ok {
			return nil, req, "", ErrDenied
		}
	}
	if req.Resource, ok = pol.Resource(r.Resource); !ok {
		return nil, req, "", ErrDenied
	}
	if r.Kind == KindStream || r.Kind == KindDatagram {
		if host, req.Port, err = allowTarget(req.Resource, r.Target); err != nil {
			return nil, req, "", err
		}
	}
	if d := pol.Decide(req); !d.Allow {
		return nil, req, "", accessDenied{d.Reasons}
	}
	return pol, req, host, nil
}

// servable checks that this gateway can serve kind on res, after the PDP: only
// an allowed user learns that the spool is full. The host guard runs on the
// host a TargetDialer dials; an approval keeps it, since the recheck refuses a
// changed config.
func (p *Pipeline) servable(ctx context.Context, kind Kind, res Resource, host string, recorded bool) error {
	if recorded && (p.spool.Total() >= p.cfg.SpoolMax || !p.spool.Writable()) {
		return ErrSpoolMax
	}
	c, ok := LookupConnector(res.Type)
	if !ok {
		return ErrUnsupportedType
	}
	if !slices.Contains(c.Kinds(), kind) {
		return ErrUnsupportedKind
	}
	if _, dials := c.(TargetDialer); dials {
		return p.guard.allowHost(ctx, dialHost(res, host))
	}
	return nil
}
