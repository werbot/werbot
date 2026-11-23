package agentic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/agentic/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// Tool call bounds.
const (
	MaxToolArguments   = 16 << 10 // bytes of the JSON arguments of one call
	MaxToolCallsPerRun = 1000
)

var (
	toolRe        = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	traceparentRe = regexp.MustCompile(`^[0-9a-f]{2}-[0-9a-f]{32}-[0-9a-f]{16}-[0-9a-f]{2}$`)
	toolStatuses  = []string{"ok", "error", "pending_approval", "denied"}
)

var (
	errNoRun            = fmt.Errorf("not an agent run token: %w", apperr.ErrPermissionDenied)
	errToolCallNotFound = fmt.Errorf("tool call not found: %w", apperr.ErrNotFound)
)

// ToolCallInput is one MCP tool call of a run. The client masks Arguments
// before it sends them.
type ToolCallInput struct {
	Tool        string
	Arguments   map[string]any
	Traceparent string // W3C trace context; anything else is stored empty
}

// RecordToolCall stores a running tool call of p's run; p must be an agent
// run principal (else ErrPermissionDenied). The run must be running and not
// expired (else ErrRunNotActive) and below MaxToolCallsPerRun calls (else
// ErrLimitExceeded). Tool calls are not audited: the table is the transcript.
func (s *Service) RecordToolCall(ctx context.Context, p principal.Principal, in ToolCallInput) (uuid.UUID, error) {
	if p.RunID == nil {
		return uuid.Nil, errNoRun
	}
	args, err := toolArguments(in)
	if err != nil {
		return uuid.Nil, err
	}
	if !traceparentRe.MatchString(in.Traceparent) {
		in.Traceparent = ""
	}
	var id uuid.UUID
	err = s.tx(ctx, p.OrgID, p.UserID, func(st Store, _ pgx.Tx) error {
		if err := toolCallGate(ctx, st, p.OrgID, *p.RunID); err != nil {
			return err
		}
		id, err = st.InsertToolCall(ctx, dbq.InsertToolCallParams{OrgID: p.OrgID, RunID: *p.RunID, Tool: in.Tool,
			Arguments: args, Traceparent: in.Traceparent})
		return err
	})
	return id, err
}

// toolArguments checks the tool name and returns the arguments as JSON.
func toolArguments(in ToolCallInput) ([]byte, error) {
	if !toolRe.MatchString(in.Tool) {
		return nil, fmt.Errorf("tool must match %s: %w", toolRe, apperr.ErrInvalidArgument)
	}
	if in.Arguments == nil {
		return []byte("{}"), nil
	}
	raw, err := json.Marshal(in.Arguments)
	if err != nil || len(raw) > MaxToolArguments {
		return nil, fmt.Errorf("tool arguments must be JSON of at most %d bytes: %w", MaxToolArguments, apperr.ErrInvalidArgument)
	}
	return raw, nil
}

// toolCallGate locks the run and refuses a call of an inactive run or one
// over MaxToolCallsPerRun.
func toolCallGate(ctx context.Context, st Store, orgID, runID uuid.UUID) error {
	active, err := st.ToolCallGate(ctx, dbq.ToolCallGateParams{OrgID: orgID, ID: runID})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return errRunNotFound
	case err != nil:
		return err
	case !active:
		return ErrRunNotActive
	}
	// A statement of its own: it sees the calls committed while this one
	// waited for the run lock.
	n, err := st.CountToolCalls(ctx, dbq.CountToolCallsParams{OrgID: orgID, RunID: runID})
	if err != nil {
		return err
	}
	if n >= MaxToolCallsPerRun {
		return ErrLimitExceeded
	}
	return nil
}

// FinishToolCall ends a running tool call of p's run with status ("ok",
// "error", "pending_approval" or "denied"). A call of another run is
// ErrNotFound, a finished call ErrFailedPrecondition.
func (s *Service) FinishToolCall(ctx context.Context, p principal.Principal, id uuid.UUID, status string) error {
	if p.RunID == nil {
		return errNoRun
	}
	if !slices.Contains(toolStatuses, status) {
		return fmt.Errorf("tool call status must be one of %v: %w", toolStatuses, apperr.ErrInvalidArgument)
	}
	return s.tx(ctx, p.OrgID, p.UserID, func(st Store, _ pgx.Tx) error {
		cur, err := st.LockToolCall(ctx, dbq.LockToolCallParams{OrgID: p.OrgID, RunID: *p.RunID, ID: id})
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return errToolCallNotFound
		case err != nil:
			return err
		case cur != "running":
			return fmt.Errorf("tool call already finished: %w", apperr.ErrFailedPrecondition)
		}
		return st.FinishToolCall(ctx, dbq.FinishToolCallParams{OrgID: p.OrgID, ID: id, Status: status})
	})
}
