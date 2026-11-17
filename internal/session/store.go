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
	UpdateSessionCounters(ctx context.Context, arg dbq.UpdateSessionCountersParams) error
	ResourceIDsByName(ctx context.Context, arg dbq.ResourceIDsByNameParams) ([]dbq.ResourceIDsByNameRow, error)
	MembersAmong(ctx context.Context, arg dbq.MembersAmongParams) ([]uuid.UUID, error)
	EndSession(ctx context.Context, arg dbq.EndSessionParams) (*uuid.UUID, error)
	SessionOfInstanceExists(ctx context.Context, arg dbq.SessionOfInstanceExistsParams) (bool, error)
	EndMissingSessions(ctx context.Context, arg dbq.EndMissingSessionsParams) ([]dbq.EndMissingSessionsRow, error)
	EndLostSessions(ctx context.Context, lostAfterSecs float64) ([]dbq.EndLostSessionsRow, error)
	GetSigningKey(ctx context.Context) (dbq.GetSigningKeyRow, error)
	InsertSigningKey(ctx context.Context, arg dbq.InsertSigningKeyParams) error
	CreateRecording(ctx context.Context, arg dbq.CreateRecordingParams) error
	LockRecording(ctx context.Context, arg dbq.LockRecordingParams) (dbq.LockRecordingRow, error)
	AppendRecordingChunk(ctx context.Context, arg dbq.AppendRecordingChunkParams) error
	FinishRecording(ctx context.Context, arg dbq.FinishRecordingParams) error
	ListSessions(ctx context.Context, arg dbq.ListSessionsParams) ([]dbq.ListSessionsRow, error)
	GetRecording(ctx context.Context, arg dbq.GetRecordingParams) (dbq.GetRecordingRow, error)
}

// newStore binds the queries to a dbx transaction.
func newStore(tx pgx.Tx) Store { return dbq.New(tx) }
