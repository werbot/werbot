package audit

import (
	"context"

	"github.com/google/uuid"
)

// Store is the persistence the use cases need.
type Store interface {
	// ListEvents returns at most PageSize+1 events of orgID, newest first.
	// The service passes a normalized filter (Page >= 1, 1 <= PageSize <= 200).
	ListEvents(ctx context.Context, orgID uuid.UUID, f Filter) ([]Event, error)
	// GetEvent returns apperr.ErrNotFound for an event of another organization.
	GetEvent(ctx context.Context, orgID, id uuid.UUID) (Event, error)
}
