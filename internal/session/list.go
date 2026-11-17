package session

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/session/dbq"
)

const (
	defaultPageSize = 50
	maxPageSize     = 200
)

// ListFilter narrows ListSessions. A member sees only their own sessions.
type ListFilter struct {
	UserID, ResourceID *uuid.UUID
	From, To           *time.Time // [From, To) by started_at
	Status             string     // "" | "active" | "ended"
	Page, PageSize     int        // Page 0 is 1; PageSize 0 is 50, above 200 is 200
}

// Summary is a session with its user, resource and recording state.
// The ids are nil once the user, resource or gateway is deleted.
type Summary struct {
	ID                                 uuid.UUID
	UserID, ResourceID, GatewayID      *uuid.UUID
	UserLogin, ResourceName, EndReason string
	ClientIP                           netip.Addr
	StartedAt                          time.Time
	EndedAt                            *time.Time
	Chunks                             int
	Bytes                              int64
	Finished                           bool
}

// ListSessions returns one page of the organization's sessions, newest first,
// and the next page number (0 after the last).
func (s *Service) ListSessions(ctx context.Context, p principal.Principal, f ListFilter) ([]Summary, int, error) {
	if f.Page < 0 || f.PageSize < 0 || (f.Status != "" && f.Status != "active" && f.Status != "ended") ||
		(f.From != nil && f.To != nil && !f.To.After(*f.From)) {
		return nil, 0, fmt.Errorf("invalid session filter: %w", apperr.ErrInvalidArgument)
	}
	if p.Role == principal.RoleMember {
		if f.UserID != nil && *f.UserID != p.UserID {
			return nil, 0, fmt.Errorf("members see only their own sessions: %w", apperr.ErrPermissionDenied)
		}
		f.UserID = &p.UserID
	}
	page, size := max(f.Page, 1), min(f.PageSize, maxPageSize)
	if size == 0 {
		size = defaultPageSize
	}
	rows, err := s.list(ctx, p, dbq.ListSessionsParams{
		OrgID: p.OrgID, UserID: f.UserID, ResourceID: f.ResourceID, FromTs: f.From, ToTs: f.To, Status: f.Status,
		RowOffset: int32((page - 1) * size), RowLimit: int32(size + 1), //nolint:gosec // page and size are bounded
	})
	if err != nil {
		return nil, 0, fmt.Errorf("session: list: %w", err)
	}
	next := 0
	if len(rows) > size { // the extra row proves a next page
		rows, next = rows[:size], page+1
	}
	out := make([]Summary, len(rows))
	for i, r := range rows {
		out[i] = summary(r)
	}
	return out, next, nil
}

// GetSession returns one session; one the principal may not see is apperr.ErrNotFound.
func (s *Service) GetSession(ctx context.Context, p principal.Principal, id uuid.UUID) (Summary, error) {
	arg := dbq.ListSessionsParams{OrgID: p.OrgID, ID: &id, RowLimit: 1}
	if p.Role == principal.RoleMember {
		arg.UserID = &p.UserID
	}
	rows, err := s.list(ctx, p, arg)
	if err != nil {
		return Summary{}, fmt.Errorf("session: get: %w", err)
	}
	if len(rows) == 0 {
		return Summary{}, fmt.Errorf("session not found: %w", apperr.ErrNotFound)
	}
	return summary(rows[0]), nil
}

func (s *Service) list(ctx context.Context, p principal.Principal, arg dbq.ListSessionsParams) (rows []dbq.ListSessionsRow, err error) {
	err = dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) (err error) {
		rows, err = newStore(tx).ListSessions(ctx, arg)
		return err
	})
	return rows, err
}

func summary(r dbq.ListSessionsRow) Summary {
	return Summary{ID: r.ID, UserID: r.UserID, ResourceID: r.ResourceID, GatewayID: r.GatewayID,
		UserLogin: r.UserLogin, ResourceName: r.ResourceName, EndReason: r.EndReason,
		ClientIP: r.ClientIp, StartedAt: r.StartedAt, EndedAt: r.EndedAt,
		Chunks: int(r.Chunks), Bytes: r.Bytes, Finished: r.Finished}
}
