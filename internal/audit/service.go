package audit

import (
	"context"
	"fmt"
	"math"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

const (
	defaultPageSize = 50
	maxPageSize     = 200
	maxPage         = math.MaxInt32 / maxPageSize // keeps (page-1)*size inside int32
)

// Service reads the audit journal.
type Service struct{ store Store }

// New returns a Service over store.
func New(store Store) *Service { return &Service{store: store} }

// ListEvents returns one page of the organization's events, newest first,
// and the next page number (0 after the last). Only owners and admins may read.
func (s *Service) ListEvents(ctx context.Context, p principal.Principal, f Filter) ([]Event, int, error) {
	if !canRead(p) {
		return nil, 0, fmt.Errorf("audit journal is for owners and admins: %w", apperr.ErrPermissionDenied)
	}
	if f.Page < 0 || f.Page > maxPage || f.PageSize < 0 || (f.From != nil && f.To != nil && !f.To.After(*f.From)) {
		return nil, 0, fmt.Errorf("invalid audit filter: %w", apperr.ErrInvalidArgument)
	}
	f.Page, f.PageSize = max(f.Page, 1), min(f.PageSize, maxPageSize)
	if f.PageSize == 0 {
		f.PageSize = defaultPageSize
	}
	events, err := s.store.ListEvents(ctx, p.OrgID, f)
	if err != nil {
		return nil, 0, fmt.Errorf("audit: list: %w", err)
	}
	next := 0
	if len(events) > f.PageSize { // the extra row proves a next page
		events, next = events[:f.PageSize], f.Page+1
	}
	return events, next, nil
}

// GetEvent returns one event; one of another organization is apperr.ErrNotFound.
func (s *Service) GetEvent(ctx context.Context, p principal.Principal, id uuid.UUID) (Event, error) {
	if !canRead(p) {
		return Event{}, fmt.Errorf("audit journal is for owners and admins: %w", apperr.ErrPermissionDenied)
	}
	e, err := s.store.GetEvent(ctx, p.OrgID, id)
	if err != nil {
		return Event{}, fmt.Errorf("audit: get: %w", err)
	}
	return e, nil
}

func canRead(p principal.Principal) bool {
	return p.Role == principal.RoleOwner || p.Role == principal.RoleAdmin
}
