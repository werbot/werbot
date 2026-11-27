package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/credential"
	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/session"
	"github.com/werbot/werbot/internal/tunnel"
)

// The refusals the gateway shows an agent: the agent credential of the
// resource is not verified, the write has no matching approval.
var (
	errAgentNotVerified = connect.NewError(connect.CodeFailedPrecondition, errors.New("agent credential not verified"))
	errWriteNotApproved = connect.NewError(connect.CodePermissionDenied, errors.New("write not approved"))
)

// tunnelCredentials reveals credentials to gateways: the primary one for a
// session, the verified agent_readonly one for an agent in a database, the
// primary one for an approved agent write, the agent_readonly one for its check.
type tunnelCredentials struct {
	c        *credential.Service
	inv      *inventory.Service
	sessions *session.Service
	acc      *access.Service
}

// Reveal returns an empty secret for a resource without a primary credential
// (tcp, udp, telnet without a password); the connector decides whether it
// needs one. A registered type that accepts no credential kind can have none
// stored (SetCredential checks the kind, the type never changes): no lookup.
// An agent's session of a type with a query action gets only the verified
// agent_readonly credential.
func (c tunnelCredentials) Reveal(ctx context.Context, orgID, resourceID uuid.UUID, resourceType string, agent bool) (tunnel.Secret, error) {
	t, ok := connector.Lookup(resourceType)
	if ok && len(t.Credentials) == 0 {
		return tunnel.Secret{}, nil
	}
	if _, query := t.QueryAction(); ok && agent && query {
		return c.agentReadonly(ctx, orgID, resourceID)
	}
	return c.primary(ctx, orgID, resourceID)
}

func (c tunnelCredentials) primary(ctx context.Context, orgID, resourceID uuid.UUID) (tunnel.Secret, error) {
	sec, err := c.c.Reveal(ctx, orgID, resourceID, credential.SlotPrimary)
	if errors.Is(err, apperr.ErrNotFound) {
		return tunnel.Secret{}, nil
	}
	if err != nil {
		return tunnel.Secret{}, err
	}
	return tunnel.Secret{Kind: string(sec.Kind), Value: sec.Value}, nil
}

// agentReadonly is the agent_readonly credential while its check is ok. It
// reads the status after the secret: a credential set meanwhile has already
// moved the status to pending in the same transaction. A missing credential
// is never an empty secret.
func (c tunnelCredentials) agentReadonly(ctx context.Context, orgID, resourceID uuid.UUID) (tunnel.Secret, error) {
	sec, err := c.c.Reveal(ctx, orgID, resourceID, credential.SlotAgentReadonly)
	if errors.Is(err, apperr.ErrNotFound) {
		return tunnel.Secret{}, errAgentNotVerified
	}
	if err != nil {
		return tunnel.Secret{}, err
	}
	r, err := c.inv.Get(ctx, orgID, resourceID)
	if err != nil {
		return tunnel.Secret{}, err
	}
	if r.AgentReadonlyStatus != "ok" {
		return tunnel.Secret{}, errAgentNotVerified
	}
	return tunnel.Secret{Kind: string(sec.Kind), Value: sec.Value}, nil
}

// WriteCredential is the primary credential for one write of an open agent
// session of gw's instance: the approval must be an approved command of the
// session's run and resource, sent through this instance, for the same SHA-256,
// decided within access.AgentApprovalReuse. Every mismatch is "write not approved".
func (c tunnelCredentials) WriteCredential(ctx context.Context, gw tunnel.Gateway, sessionID, approvalID uuid.UUID, opSHA256 []byte) (tunnel.Secret, error) {
	run, resource, err := c.sessions.OpenAgentSession(ctx, gw.OrgID, gw.InstanceID, sessionID)
	if errors.Is(err, apperr.ErrNotFound) {
		return tunnel.Secret{}, errWriteNotApproved
	}
	if err != nil {
		return tunnel.Secret{}, err
	}
	a, err := c.acc.GetApproval(ctx, gw.OrgID, approvalID)
	if errors.Is(err, apperr.ErrNotFound) {
		return tunnel.Secret{}, errWriteNotApproved
	}
	if err != nil {
		return tunnel.Secret{}, err
	}
	if !coversWrite(a, gw.InstanceID, run, resource, opSHA256) {
		return tunnel.Secret{}, errWriteNotApproved
	}
	return c.primary(ctx, gw.OrgID, resource)
}

// coversWrite reports whether a allows the write opSHA256 of run on resource
// through instance now.
func coversWrite(a access.Approval, instance, run, resource uuid.UUID, opSHA256 []byte) bool {
	return a.Kind == access.KindCommand && a.Status == access.StatusApproved &&
		a.DecidedAt != nil && time.Since(*a.DecidedAt) < access.AgentApprovalReuse &&
		is(a.InstanceID, instance) && is(a.AgentRunID, run) && is(a.ResourceID, resource) && bytes.Equal(a.CommandSHA256, opSHA256)
}

// is reports whether p is set to id.
func is(p *uuid.UUID, id uuid.UUID) bool { return p != nil && *p == id }

// ProbeCredential is the agent_readonly credential of a resource of gw's pool
// whose check status is not none, with the check version: the later of the
// credential's and the resource's updated_at, in microseconds. A missing
// credential is NotFound, never an empty secret; no audit (as a heartbeat).
func (c tunnelCredentials) ProbeCredential(ctx context.Context, gw tunnel.Gateway, resourceID uuid.UUID) (tunnel.Secret, int64, error) {
	sec, err := c.c.Reveal(ctx, gw.OrgID, resourceID, credential.SlotAgentReadonly)
	if err != nil {
		return tunnel.Secret{}, 0, err
	}
	r, err := c.inv.Get(ctx, gw.OrgID, resourceID)
	if err != nil {
		return tunnel.Secret{}, 0, err
	}
	if r.GatewayID == nil || *r.GatewayID != gw.ID || r.AgentReadonlyStatus == "none" {
		return tunnel.Secret{}, 0, fmt.Errorf("resource not found: %w", apperr.ErrNotFound)
	}
	return tunnel.Secret{Kind: string(sec.Kind), Value: sec.Value}, max(sec.Version, r.UpdatedAt.UnixMicro()), nil
}
