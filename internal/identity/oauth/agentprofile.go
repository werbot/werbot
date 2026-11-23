package oauth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/identity/oauth/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// An agent profile is a cli_session bound to one agent: its refresh token
// (rotated as any) gives access tokens of aud AudienceAgentExchange, good
// only for the token exchange of a run of that agent. A leaked profile acts
// as the agent, never as its sponsor.

const (
	// MaxAgentProfiles caps the live profiles of one agent.
	MaxAgentProfiles = 5
	// AgentProfileMinTTL is the shortest profile lifetime; the longest is the
	// organization's cli_session_max.
	AgentProfileMinTTL = time.Hour
)

var (
	errProfileAgentToken = fmt.Errorf("not allowed for agent tokens: %w", apperr.ErrPermissionDenied)
	errProfileAgent      = fmt.Errorf("agent not found: %w", apperr.ErrNotFound)
	errProfileNotFound   = fmt.Errorf("agent profile not found: %w", apperr.ErrNotFound)
	errProfileFrozen     = fmt.Errorf("agent is frozen: %w", apperr.ErrFailedPrecondition)
	errProfileLimit      = fmt.Errorf("agent has %d live profiles; revoke one first: %w", MaxAgentProfiles, apperr.ErrFailedPrecondition)
	errProfileTTL        = fmt.Errorf("ttl must be 0 or from 1h to the organization's cli_session_max: %w", apperr.ErrInvalidArgument)
)

// AgentProfile is a live agent profile; Label is its hostname column.
type AgentProfile struct {
	ID, AgentID, SponsorID                              uuid.UUID
	AgentName, Label                                    string
	CreatedAt, LastUsedAt, ExpiresAt, AbsoluteExpiresAt time.Time
}

// person refuses an agent run token: agents never manage profiles.
func person(pr principal.Principal) error {
	if pr.IsAgent() {
		return errProfileAgentToken
	}
	return nil
}

// CreateAgentProfile opens a profile of the caller's own agent (owner and
// admin included: another sponsor's agent is ErrNotFound) and returns it with
// its refresh token, shown only here. ttl 0 is cli_session_max; a frozen
// agent and one with MaxAgentProfiles live profiles are refused. Audit
// agent.profile.create, without the token.
func (p *Provider) CreateAgentProfile(ctx context.Context, pr principal.Principal, agentID uuid.UUID, ttl time.Duration, label string) (AgentProfile, string, error) {
	if err := person(pr); err != nil {
		return AgentProfile{}, "", err
	}
	raw, hash := newRefreshToken()
	out := AgentProfile{AgentID: agentID, SponsorID: pr.UserID, Label: cleanHostname(label)}
	err := dbx.InOrg(ctx, p.pool, pr.OrgID, pr.UserID, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		name, ttl, err := lockProfileAgent(ctx, q, pr, agentID, ttl)
		if err != nil {
			return err
		}
		ri := &reqInfo{ip: clientip.From(ctx)} // a Connect request: only its address
		r, err := q.CreateAgentProfile(ctx, dbq.CreateAgentProfileParams{UserID: pr.UserID, Hostname: out.Label,
			Ip: ri.ipPtr(), RefreshHash: hash, AgentID: agentID, TtlSecs: ttl.Seconds(), OrgID: pr.OrgID})
		if err != nil {
			return err
		}
		out.ID, out.AgentName, out.CreatedAt, out.LastUsedAt, out.ExpiresAt, out.AbsoluteExpiresAt =
			r.ID, name, r.CreatedAt, r.LastUsedAt, r.ExpiresAt, r.AbsoluteExpiresAt
		return auditlog.Write(ctx, tx, auditlog.Event{
			OrgID: &pr.OrgID, ActorUserID: &pr.UserID, ActorKind: "user", Action: "agent.profile.create",
			TargetType: "agent_profile", TargetID: r.ID.String(), IP: ri.ip,
			Data: map[string]any{"agent_id": agentID, "label": out.Label, "expires_at": r.AbsoluteExpiresAt},
		})
	})
	if err != nil {
		return AgentProfile{}, "", err
	}
	return out, raw, nil
}

// lockProfileAgent locks pr's own active agent for a new profile, checks
// its live profiles and resolves ttl (0: cli_session_max); it returns the
// agent's name. The count runs after the lock: concurrent creates of one
// agent queue on it and each sees the profiles of those before.
func lockProfileAgent(ctx context.Context, q *dbq.Queries, pr principal.Principal, agentID uuid.UUID, ttl time.Duration) (string, time.Duration, error) {
	row, err := q.LockProfileAgent(ctx, dbq.LockProfileAgentParams{ID: agentID, OrgID: pr.OrgID})
	switch {
	case errors.Is(err, pgx.ErrNoRows) || (err == nil && row.SponsorUserID != pr.UserID):
		return "", 0, errProfileAgent
	case err != nil:
		return "", 0, err
	case row.Status != "active":
		return "", 0, errProfileFrozen
	}
	live, err := q.CountAgentProfiles(ctx, dbq.CountAgentProfilesParams{OrgID: pr.OrgID, AgentID: agentID})
	if err != nil {
		return "", 0, err
	}
	if live >= MaxAgentProfiles {
		return "", 0, errProfileLimit
	}
	ttl, err = profileTTL(ttl, time.Duration(row.MaxSecs)*time.Second)
	return row.Name, ttl, err
}

// profileTTL resolves a requested lifetime: 0 is maxTTL (cli_session_max),
// otherwise AgentProfileMinTTL..maxTTL.
func profileTTL(ttl, maxTTL time.Duration) (time.Duration, error) {
	switch {
	case ttl == 0:
		return maxTTL, nil
	case ttl < AgentProfileMinTTL || ttl > maxTTL:
		return 0, errProfileTTL
	}
	return ttl, nil
}

// ListAgentProfiles lists live profiles, newest first: a member's own,
// every one of the organization for owner and admin; of agentID if set.
func (p *Provider) ListAgentProfiles(ctx context.Context, pr principal.Principal, agentID *uuid.UUID) ([]AgentProfile, error) {
	if err := person(pr); err != nil {
		return nil, err
	}
	arg := dbq.ListAgentProfilesParams{OrgID: pr.OrgID, AgentID: agentID}
	if !pr.Manager() {
		arg.SponsorUserID = &pr.UserID
	}
	var out []AgentProfile
	err := dbx.InOrg(ctx, p.pool, pr.OrgID, pr.UserID, func(tx pgx.Tx) error {
		rows, err := dbq.New(tx).ListAgentProfiles(ctx, arg)
		for _, r := range rows {
			out = append(out, AgentProfile{ID: r.ID, AgentID: r.AgentID, SponsorID: r.UserID, AgentName: r.AgentName, Label: r.Hostname,
				CreatedAt: r.CreatedAt, LastUsedAt: r.LastUsedAt, ExpiresAt: r.ExpiresAt, AbsoluteExpiresAt: r.AbsoluteExpiresAt})
		}
		return err
	})
	return out, err
}

// RevokeAgentProfile ends profile id: the caller's own, any of the
// organization for owner and admin (audit agent.profile.revoke, reason
// revoked). A profile that is not there, not the caller's or already
// revoked is ErrNotFound.
func (p *Provider) RevokeAgentProfile(ctx context.Context, pr principal.Principal, id uuid.UUID) error {
	if err := person(pr); err != nil {
		return err
	}
	return dbx.InOrg(ctx, p.pool, pr.OrgID, pr.UserID, func(tx pgx.Tx) error {
		agentID, err := dbq.New(tx).RevokeAgentProfile(ctx, dbq.RevokeAgentProfileParams{ID: id, OrgID: pr.OrgID, Manager: pr.Manager(), UserID: pr.UserID})
		if errors.Is(err, pgx.ErrNoRows) {
			return errProfileNotFound
		}
		if err != nil {
			return err
		}
		return revokeAudit(ctx, tx, pr.OrgID, pr.UserID, id, *agentID, "revoked")
	})
}

// RevokeAgentProfiles ends every profile of agentID in tx (the caller's
// transaction, agentic freezing the agent), one agent.profile.revoke each by
// actor. It needs no Provider: agentic is built first.
func RevokeAgentProfiles(ctx context.Context, tx pgx.Tx, orgID, agentID, actor uuid.UUID, reason string) error {
	ids, err := dbq.New(tx).RevokeProfilesOfAgent(ctx, dbq.RevokeProfilesOfAgentParams{Reason: reason, OrgID: orgID, AgentID: agentID})
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := revokeAudit(ctx, tx, orgID, actor, id, agentID, reason); err != nil {
			return err
		}
	}
	return nil
}

func revokeAudit(ctx context.Context, tx pgx.Tx, orgID, actor, id, agentID uuid.UUID, reason string) error {
	return auditlog.Write(ctx, tx, auditlog.Event{
		OrgID: &orgID, ActorUserID: &actor, ActorKind: "user", Action: "agent.profile.revoke",
		TargetType: "agent_profile", TargetID: id.String(), IP: clientip.From(ctx),
		Data: map[string]any{"agent_id": agentID, "reason": reason},
	})
}
