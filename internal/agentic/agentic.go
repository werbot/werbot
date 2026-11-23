// Package agentic owns AI agents: an agent acts for its sponsor (a member of
// the organization) inside bounded runs with an action budget, a concurrency
// cap and a hard expiry.
package agentic

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/internal/agentic/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/dbx"
)

// Run defaults (agent.limits overrides them) and the pause threshold.
const (
	DefaultMaxActions    = 100
	DefaultMaxDuration   = time.Hour
	DefaultMaxConcurrent = 4
	PauseAfterDenials    = 5 // consecutive denials that pause a run
)

// Errors of a run that refuses an action; both map to FailedPrecondition.
var (
	ErrLimitExceeded = fmt.Errorf("agent limit exceeded: %w", apperr.ErrFailedPrecondition)
	ErrRunNotActive  = fmt.Errorf("agent run is not active: %w", apperr.ErrFailedPrecondition)
)

var (
	errFrozen = fmt.Errorf("agent is frozen: %w", apperr.ErrPermissionDenied)
	errLocked = fmt.Errorf("agent is locked: %w", apperr.ErrPermissionDenied)
)

// Agent is an agent of a sponsor; Name is unique per sponsor.
type Agent struct {
	ID, OrgID, SponsorID uuid.UUID
	Name, Kind, Status   string // Kind "interactive"; Status "active" | "frozen"
	Limits               Limits // zero fields: the defaults
	CreatedAt            time.Time
}

// Run is one working period of an agent. MaxActions and MaxConcurrent are
// copied from the agent's limits at start: a later limits change never alters it.
type Run struct {
	ID, OrgID, AgentID, SponsorID uuid.UUID
	Client, Reason, Status        string // Status "running" | "paused" | "ended"
	Actions, DenialsInRow         int
	MaxActions, MaxConcurrent     int
	ExpiresAt, StartedAt          time.Time
	EndedAt                       *time.Time
	EndReason                     string // "client_exit" | "ended_by_user" | "expired"; "" while not ended
}

// SnapshotAgent is an agent as the policy snapshot needs it.
type SnapshotAgent struct {
	ID, SponsorID uuid.UUID
	Status        string
}

// EmailQueue queues an email inside the caller's transaction.
type EmailQueue interface {
	EnqueueTx(ctx context.Context, tx pgx.Tx, to, template string, data map[string]string) error
}

// Config configures a Service.
type Config struct {
	WebURL string // base of the run link in agent_alert emails
}

// Service manages the agents and runs of organizations.
type Service struct {
	pool   *pgxpool.Pool
	emails EmailQueue
	cfg    Config
}

// New creates a Service.
func New(pool *pgxpool.Pool, emails EmailQueue, cfg Config) *Service {
	return &Service{pool: pool, emails: emails, cfg: cfg}
}

func (s *Service) tx(ctx context.Context, orgID, userID uuid.UUID, fn func(Store, pgx.Tx) error) error {
	return dbx.InOrg(ctx, s.pool, orgID, userID, func(tx pgx.Tx) error { return fn(newStore(tx), tx) })
}

// audit writes an event of an actor of kind ("user" or "system"; actor nil for system).
func audit(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, kind string, actor *uuid.UUID, action, targetType string, id uuid.UUID, data map[string]any) error {
	return auditlog.Write(ctx, tx, auditlog.Event{OrgID: &orgID, ActorUserID: actor, ActorKind: kind,
		Action: action, TargetType: targetType, TargetID: id.String(), IP: clientip.From(ctx), Data: data})
}

func toAgent(r dbq.Agent) (Agent, error) {
	l, err := parseLimits(r.Limits)
	if err != nil {
		return Agent{}, fmt.Errorf("agentic: limits of agent %s: %w", r.ID, err)
	}
	return Agent{ID: r.ID, OrgID: r.OrgID, SponsorID: r.SponsorUserID, Name: r.Name, Kind: r.Kind,
		Status: r.Status, Limits: l, CreatedAt: r.CreatedAt}, nil
}

func toRun(r dbq.AgentRun) Run {
	run := Run{ID: r.ID, OrgID: r.OrgID, AgentID: r.AgentID, SponsorID: r.SponsorUserID, Client: r.Client,
		Reason: r.Reason, Status: r.Status, Actions: int(r.Actions), DenialsInRow: int(r.DenialsInRow),
		MaxActions: int(r.MaxActions), MaxConcurrent: int(r.MaxConcurrent), ExpiresAt: r.ExpiresAt,
		StartedAt: r.StartedAt, EndedAt: r.EndedAt}
	if r.EndReason != nil {
		run.EndReason = *r.EndReason
	}
	return run
}
