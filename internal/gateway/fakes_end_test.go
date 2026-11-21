package gateway

import (
	"context"

	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/gateway/recorder"
)

// EndSession records a delivered end.
func (f *fakeControl) EndSession(_ context.Context, id uuid.UUID, reason string, c *gatewayv1.SessionCounters, r *recorder.SessionResult) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.upErr != nil {
		return f.upErr
	}
	noteResult(id, r)
	f.ends = append(f.ends, endCall{id, reason, c})
	f.events = append(f.events, "end:"+reason)
	return nil
}
