package gateway

import (
	"context"
	"time"
)

// handshakeReuse is how long after a successful SSH handshake a channel of
// the same connection takes the handshake user instead of a second
// AuthenticateUser. A key revoked within this window of the handshake cannot
// be told apart from one revoked just after the channel opened, and that
// channel keeps its session anyway; later channels (a port forward minutes
// later, the menu, a reconnect) authenticate again. A reused user takes its
// teams and organization membership from the snapshot the channel is decided
// on, as the sweeper's recheck does, so a team removal is not stretched past
// the snapshot window; the PDP and target checks run on every channel either way.
const handshakeReuse = 2 * time.Second

// Handshake is the user a front authenticated for the request's identity when
// the connection was set up, and when.
type Handshake struct {
	User User
	At   time.Time
}

// user returns the user of r and whether it is the handshake user: that one
// while it is at most handshakeReuse old, otherwise the control plane's answer.
func (p *Pipeline) user(ctx context.Context, r OpenRequest) (u User, reused bool, err error) {
	if h := r.Handshake; h != nil && p.now().Sub(h.At) <= handshakeReuse {
		return h.User, true, nil
	}
	u, err = p.ctl.AuthenticateUser(ctx, r.Identity.Value, r.ClientIP)
	return u, false, err
}
