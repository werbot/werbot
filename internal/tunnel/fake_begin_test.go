package tunnel

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// Begin opens no recording in the shared fake.
func (r *fakeIngest) Begin(context.Context, uuid.UUID, uuid.UUID) error { return nil }

// beginIngest records the Begin calls of a fakeIngest and returns err from each.
type beginIngest struct {
	*fakeIngest
	mu     sync.Mutex
	err    error
	begins [][2]uuid.UUID // (org, session)
}

func (r *beginIngest) Begin(_ context.Context, orgID, sessionID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.begins = append(r.begins, [2]uuid.UUID{orgID, sessionID})
	return r.err
}
