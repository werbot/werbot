package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/platform/tlsca"
	"github.com/werbot/werbot/internal/platform/version"
	"github.com/werbot/werbot/internal/relay"
	"github.com/werbot/werbot/internal/session"
	"github.com/werbot/werbot/internal/tunnel"
)

// newRelay is the relay of this replica under a new instance id; newServices
// builds it before jobs.NewClient, which takes its GC job.
func newRelay(cfg config.Config, pool *pgxpool.Pool, ca *tlsca.CA, tickets relay.Tickets, reg *metrics.Registry, logger *slog.Logger) *relay.Relay {
	return relay.New(pool, ca, tickets, relay.Config{InstanceID: uuid.Must(uuid.NewV7()), InternalAddr: cfg.InternalAddr,
		InternalURL: cfg.InternalURL, Version: version.Version, WebURL: cfg.WebURL, Metrics: reg}, logger)
}

// startRelay runs r until ctx ends; a failure (its listener, the registry)
// goes to errc, which must have room for it. The channel closes once Run returned.
func startRelay(ctx context.Context, r *relay.Relay, errc chan<- error) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := r.Run(ctx); err != nil {
			errc <- fmt.Errorf("relay: %w", err)
		}
	}()
	return done
}

// ticketsAdapter is session.Service as relay.Tickets.
type ticketsAdapter struct{ s *session.Service }

func (a ticketsAdapter) CreateObserve(ctx context.Context, p principal.Principal, sessionID, target uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	t, err := a.s.CreateObserveTicket(ctx, p, sessionID, target)
	return t.ID, t.InstanceID, err
}

func (a ticketsAdapter) CreateTerminal(ctx context.Context, p principal.Principal, resource string, cols, rows int, target uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	t, err := a.s.CreateTerminalTicket(ctx, p, resource, cols, rows, target)
	return t.ID, t.InstanceID, err
}

func (a ticketsAdapter) Consume(ctx context.Context, ticketID uuid.UUID) (relay.Ticket, error) {
	t, err := a.s.ConsumeStreamTicket(ctx, ticketID)
	return relay.Ticket{ID: t.ID, OrgID: t.OrgID, UserID: t.UserID, InstanceID: t.InstanceID, TargetServerID: t.TargetServerID,
		Mode: t.Mode, SessionID: t.SessionID, ResourceID: t.ResourceID, Cols: t.Cols, Rows: t.Rows}, err
}

// bridger is relay.Relay as tunnel.Bridger: the calling gateway instance
// must be the one the ticket is addressed to.
type bridger struct{ r *relay.Relay }

func (b bridger) Bridge(ctx context.Context, gw tunnel.Gateway, ticketID uuid.UUID, p tunnel.Pipe) error {
	return b.r.Bridge(relay.WithCaller(ctx, gw.OrgID, gw.InstanceID), ticketID, p)
}

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
