package tunnel

import (
	"context"

	"connectrpc.com/connect"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// slotAgentReadonly is the only slot GetProbeCredential serves: the
// read-only check needs nothing else (Ruling C15).
const slotAgentReadonly = "agent_readonly"

// GetWriteCredential returns the primary credential for one approved write of
// an open agent session of the caller; the server checks the binding.
func (h *Handler) GetWriteCredential(ctx context.Context, req *connect.Request[gatewayv1.GetWriteCredentialRequest]) (*connect.Response[gatewayv1.GetWriteCredentialResponse], error) {
	g, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	sessionID, err := parseID(req.Msg.GetSessionId(), "session_id")
	if err != nil {
		return nil, err
	}
	approvalID, err := parseID(req.Msg.GetApprovalId(), "approval_id")
	if err != nil {
		return nil, err
	}
	sec, err := h.d.Credentials.WriteCredential(ctx, g, sessionID, approvalID, req.Msg.GetOpSha256())
	if err != nil {
		return nil, h.fail(ctx, "write credential", err)
	}
	return connect.NewResponse(&gatewayv1.GetWriteCredentialResponse{Credential: credentialOf(sec)}), nil
}

// GetProbeCredential returns the agent_readonly credential of a resource of
// the caller's pool with its check version; no audit, as a heartbeat.
func (h *Handler) GetProbeCredential(ctx context.Context, req *connect.Request[gatewayv1.GetProbeCredentialRequest]) (*connect.Response[gatewayv1.GetProbeCredentialResponse], error) {
	g, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	resourceID, err := parseID(req.Msg.GetResourceId(), "resource_id")
	if err != nil {
		return nil, err
	}
	if req.Msg.GetSlot() != slotAgentReadonly {
		return nil, invalid("slot")
	}
	sec, version, err := h.d.Credentials.ProbeCredential(ctx, g, resourceID)
	if err != nil {
		return nil, h.fail(ctx, "probe credential", err)
	}
	return connect.NewResponse(&gatewayv1.GetProbeCredentialResponse{Credential: credentialOf(sec), Version: version}), nil
}

func credentialOf(s Secret) *gatewayv1.Credential {
	return &gatewayv1.Credential{Kind: s.Kind, Value: s.Value}
}
