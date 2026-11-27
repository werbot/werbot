package inventory

import (
	"context"
	"slices"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/platform/apperr"
)

// Credential slots, repeated from internal/credential: domains never import each other.
const (
	slotPrimary       = "primary"
	slotAgentReadonly = "agent_readonly"
)

// Credentials is the write-only credential store; *credential.Service satisfies it.
type Credentials interface {
	Set(ctx context.Context, orgID, resourceID uuid.UUID, slot string, kind connector.CredentialKind, value []byte) error
	Clear(ctx context.Context, orgID, resourceID uuid.UUID, slot string) error
	Kinds(ctx context.Context, orgID uuid.UUID, resourceIDs []uuid.UUID, slot string) (map[uuid.UUID]connector.CredentialKind, error)
}

// credentialSlot maps "" to the primary slot; agent_readonly exists only for
// types with a query action.
func credentialSlot(slot string, t connector.Type) (string, error) {
	switch slot {
	case "", slotPrimary:
		return slotPrimary, nil
	case slotAgentReadonly:
		if _, ok := t.QueryAction(); !ok {
			return "", invalid("resource type has no agent credential slot")
		}
		return slot, nil
	}
	return "", invalid("unknown credential slot")
}

// credentialTarget returns the resource of a credential request, its type
// (zero when no longer registered) and the slot.
func (h *Handler) credentialTarget(ctx context.Context, org uuid.UUID, resourceID, slot string) (Resource, connector.Type, string, error) {
	id, err := parseID(resourceID)
	if err != nil {
		return Resource{}, connector.Type{}, "", err
	}
	r, err := h.svc.Get(ctx, org, id)
	if err != nil {
		return Resource{}, connector.Type{}, "", err
	}
	t, _ := connector.Lookup(r.Type)
	slot, err = credentialSlot(slot, t)
	return r, t, slot, err
}

// SetCredential stores a credential in a slot; its kind must be accepted by the resource type.
func (h *Handler) SetCredential(ctx context.Context, req *connect.Request[werbotv1.SetCredentialRequest]) (*connect.Response[werbotv1.SetCredentialResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	r, t, slot, err := h.credentialTarget(ctx, org, req.Msg.GetResourceId(), req.Msg.GetSlot())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	kind := connector.CredentialKind(req.Msg.GetKind())
	if !slices.Contains(t.Credentials, kind) {
		return nil, apperr.ToConnect(invalid("credential kind %q is not accepted by type %q", kind, r.Type))
	}
	if err := h.creds.Set(ctx, org, r.ID, slot, kind, req.Msg.GetValue()); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.SetCredentialResponse{}), nil
}

// ClearCredential removes the credential stored in a slot.
func (h *Handler) ClearCredential(ctx context.Context, req *connect.Request[werbotv1.ClearCredentialRequest]) (*connect.Response[werbotv1.ClearCredentialResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	r, _, slot, err := h.credentialTarget(ctx, org, req.Msg.GetResourceId(), req.Msg.GetSlot())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.creds.Clear(ctx, org, r.ID, slot); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.ClearCredentialResponse{}), nil
}
