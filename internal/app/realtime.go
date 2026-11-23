package app

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/tunnel"
)

// ByWebTicket resolves an unexpired stream ticket of orgID addressed to
// instanceID, spent or not (the gateway authenticates it before the relay
// spends it), to its member with teams. A member whose login now names
// another user is apperr.ErrNotFound.
func (u tunnelUsers) ByWebTicket(ctx context.Context, orgID, instanceID, ticketID uuid.UUID) (tunnel.User, tunnel.WebTicket, error) {
	t, err := u.sessions.LookupStreamTicket(ctx, orgID, instanceID, ticketID)
	if err != nil {
		return tunnel.User{}, tunnel.WebTicket{}, err
	}
	usr, err := tunnelUser(u.ids.LookupLogin(ctx, orgID, t.Login))
	if err != nil {
		return tunnel.User{}, tunnel.WebTicket{}, err
	}
	if usr.ID != t.UserID {
		return tunnel.User{}, tunnel.WebTicket{}, fmt.Errorf("stream ticket user changed: %w", apperr.ErrNotFound)
	}
	return usr, tunnel.WebTicket{Mode: t.Mode, Resource: t.ResourceName, SessionID: t.SessionID,
		Cols: t.Cols, Rows: t.Rows, ClientIP: t.ClientIP}, nil
}
