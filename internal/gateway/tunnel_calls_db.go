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
