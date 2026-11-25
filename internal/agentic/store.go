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
	ActionGate(ctx context.Context, arg dbq.ActionGateParams) (dbq.ActionGateRow, error)
	OpenRunSessions(ctx context.Context, arg dbq.OpenRunSessionsParams) (int64, error)
	CountAction(ctx context.Context, arg dbq.CountActionParams) error
	SetDenials(ctx context.Context, arg dbq.SetDenialsParams) error
	ResourceIDByName(ctx context.Context, arg dbq.ResourceIDByNameParams) (uuid.UUID, error)
	AlertNames(ctx context.Context, arg dbq.AlertNamesParams) (dbq.AlertNamesRow, error)
	AlertRecipients(ctx context.Context, arg dbq.AlertRecipientsParams) ([]string, error)
	ToolCallGate(ctx context.Context, arg dbq.ToolCallGateParams) (bool, error)
	CountToolCalls(ctx context.Context, arg dbq.CountToolCallsParams) (int64, error)
	InsertToolCall(ctx context.Context, arg dbq.InsertToolCallParams) (uuid.UUID, error)
	LockToolCall(ctx context.Context, arg dbq.LockToolCallParams) (string, error)
	FinishToolCall(ctx context.Context, arg dbq.FinishToolCallParams) error
	ListAgents(ctx context.Context, arg dbq.ListAgentsParams) ([]dbq.ListAgentsRow, error)
	LockAgent(ctx context.Context, arg dbq.LockAgentParams) (dbq.Agent, error)
	UpdateAgent(ctx context.Context, arg dbq.UpdateAgentParams) error
	DeleteAgent(ctx context.Context, arg dbq.DeleteAgentParams) error
	ListRuns(ctx context.Context, arg dbq.ListRunsParams) ([]dbq.ListRunsRow, error)
	RunToolCalls(ctx context.Context, arg dbq.RunToolCallsParams) ([]dbq.RunToolCallsRow, error)
	RunSessions(ctx context.Context, arg dbq.RunSessionsParams) ([]dbq.RunSessionsRow, error)
	RunApprovals(ctx context.Context, arg dbq.RunApprovalsParams) ([]dbq.RunApprovalsRow, error)
	RunDenials(ctx context.Context, arg dbq.RunDenialsParams) ([]dbq.RunDenialsRow, error)
}
