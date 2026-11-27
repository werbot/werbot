package gateway

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// GetWriteCredential reveals the primary credential for one approved write
// of agent session sessionID: the approval must cover opSHA, the SHA-256 of
// the approved text.
func (c *tunnelClient) GetWriteCredential(ctx context.Context, sessionID uuid.UUID, approvalID string, opSHA [32]byte) (Secret, error) {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	resp, err := c.conn.Load().rpc.GetWriteCredential(ctx, connect.NewRequest(&gatewayv1.GetWriteCredentialRequest{
		SessionId: sessionID.String(), ApprovalId: approvalID, OpSha256: opSHA[:],
	}))
	if err != nil {
		return Secret{}, err
	}
	cred := resp.Msg.GetCredential()
	if cred == nil {
		return Secret{}, errors.New("get write credential: no credential in response")
	}
	return Secret{Kind: cred.GetKind(), Value: cred.GetValue()}, nil
}

// GetProbeCredential reveals the agent_readonly credential of resourceID for
// its read-only check, with the check version. configVersion is the
// resource's in the snapshot the check reads; the server refuses another.
func (c *tunnelClient) GetProbeCredential(ctx context.Context, resourceID uuid.UUID, configVersion int64) (Secret, int64, error) {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	resp, err := c.conn.Load().rpc.GetProbeCredential(ctx, connect.NewRequest(&gatewayv1.GetProbeCredentialRequest{
		ResourceId: resourceID.String(), Slot: "agent_readonly", ConfigVersion: configVersion,
	}))
	if err != nil {
		return Secret{}, 0, err
	}
	cred := resp.Msg.GetCredential()
	if len(cred.GetValue()) == 0 {
		return Secret{}, 0, errors.New("get probe credential: no credential in response")
	}
	return Secret{Kind: cred.GetKind(), Value: cred.GetValue()}, resp.Msg.GetVersion(), nil
}
