package session_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/jobs"
	"github.com/werbot/werbot/internal/session"
)

// The GC deletes tickets expired for more than 5 minutes and keeps the rest;
// RegisterJobs schedules it as stream_ticket.gc in the maintenance queue.
func TestGCStreamTickets(t *testing.T) {
	e := newTicketEnv(t)
	old, recent, live := e.terminal(t, e.a.MemberID), e.terminal(t, e.a.MemberID), e.terminal(t, e.a.MemberID)
	e.expire(t, old.ID, "6 minutes")
	e.expire(t, recent.ID, "4 minutes")

	n, err := e.svc.GCStreamTickets(t.Context())
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
	assert.Zero(t, e.count(t, "SELECT count(*) FROM stream_ticket WHERE id = $1", old.ID))
	assert.Equal(t, 2, e.count(t, "SELECT count(*) FROM stream_ticket WHERE id = ANY($1)", []uuid.UUID{recent.ID, live.ID}))

	workers := river.NewWorkers()
	c, err := jobs.NewClient(e.pool, workers, session.RegisterJobs(workers, e.svc, time.Now()), nil)
	require.NoError(t, err)
	assert.True(t, c.PeriodicJobs().RemoveByID("stream_ticket.gc"))
	assert.Equal(t, "stream_ticket.gc", session.StreamTicketGCArgs{}.Kind())
	assert.Equal(t, jobs.QueueMaintenance, session.StreamTicketGCArgs{}.InsertOpts().Queue)
}
