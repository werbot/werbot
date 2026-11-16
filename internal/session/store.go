package session

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/session/dbq"
)

// Store is the persistence the use cases need.
type Store interface {
	StartSession(ctx context.Context, arg dbq.StartSessionParams) (uuid.UUID, error)
	EndSession(ctx context.Context, arg dbq.EndSessionParams) (*uuid.UUID, error)
	SessionOfInstanceExists(ctx context.Context, arg dbq.SessionOfInstanceExistsParams) (bool, error)
	EndMissingSessions(ctx context.Context, arg dbq.EndMissingSessionsParams) ([]dbq.EndMissingSessionsRow, error)
	EndLostSessions(ctx context.Context, lostAfterSecs float64) ([]dbq.EndLostSessionsRow, error)
}

// newStore binds the queries to a dbx transaction.
func newStore(tx pgx.Tx) Store { return dbq.New(tx) }
