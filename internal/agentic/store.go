package agentic

import (
	"context"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/agentic/dbq"
)

// Store is the persistence the use cases need.
type Store interface {
	InsertAgent(ctx context.Context, arg dbq.InsertAgentParams) (dbq.Agent, error)
	GetAgentByName(ctx context.Context, arg dbq.GetAgentByNameParams) (dbq.Agent, error)
	AgentGate(ctx context.Context, arg dbq.AgentGateParams) (dbq.AgentGateRow, error)
	InsertRun(ctx context.Context, arg dbq.InsertRunParams) (dbq.AgentRun, error)
	RunGate(ctx context.Context, arg dbq.RunGateParams) (dbq.RunGateRow, error)
	LockRun(ctx context.Context, arg dbq.LockRunParams) (dbq.AgentRun, error)
	EndRun(ctx context.Context, arg dbq.EndRunParams) error
	ResumeRun(ctx context.Context, arg dbq.ResumeRunParams) error
	ExpireRuns(ctx context.Context) ([]dbq.ExpireRunsRow, error)
	SnapshotAgents(ctx context.Context, orgID uuid.UUID) ([]dbq.SnapshotAgentsRow, error)
	BlockedRuns(ctx context.Context, orgID uuid.UUID) ([]uuid.UUID, error)
}
