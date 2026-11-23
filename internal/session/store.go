package session

import (
	"context"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/session/dbq"
)

// Store is the persistence the use cases need.
type Store interface {
	UpdateSessionCounters(ctx context.Context, arg dbq.UpdateSessionCountersParams) error
	ResourceIDsByName(ctx context.Context, arg dbq.ResourceIDsByNameParams) ([]dbq.ResourceIDsByNameRow, error)
	MembersAmong(ctx context.Context, arg dbq.MembersAmongParams) ([]uuid.UUID, error)
	EndSession(ctx context.Context, arg dbq.EndSessionParams) (*uuid.UUID, error)
	SessionOfInstanceExists(ctx context.Context, arg dbq.SessionOfInstanceExistsParams) (bool, error)
	SessionInOrg(ctx context.Context, arg dbq.SessionInOrgParams) (bool, error)
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
	LockSessionToTerminate(ctx context.Context, arg dbq.LockSessionToTerminateParams) (dbq.LockSessionToTerminateRow, error)
	RequestTerminate(ctx context.Context, arg dbq.RequestTerminateParams) error
	NotifySessionTerminate(ctx context.Context, payload string) error
	PendingTerminations(ctx context.Context, arg dbq.PendingTerminationsParams) ([]uuid.UUID, error)
	ObservableSession(ctx context.Context, arg dbq.ObservableSessionParams) (uuid.UUID, error)
	TerminalTarget(ctx context.Context, arg dbq.TerminalTargetParams) (dbq.TerminalTargetRow, error)
	InsertStreamTicket(ctx context.Context, arg dbq.InsertStreamTicketParams) (dbq.StreamTicket, error)
	ConsumeStreamTicket(ctx context.Context, id uuid.UUID) (dbq.StreamTicket, error)
	LookupStreamTicket(ctx context.Context, arg dbq.LookupStreamTicketParams) (dbq.LookupStreamTicketRow, error)
	DeleteExpiredStreamTickets(ctx context.Context, afterSecs float64) (int64, error)
}

// newStore binds the queries to a dbx transaction or a dbx.Read.
func newStore(db dbq.DBTX) Store { return dbq.New(db) }
