package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/gateway/recorder"
)

// ErrHostKeyMismatch is a target host key that differs from the pinned one.
var ErrHostKeyMismatch = errors.New("SSH host key mismatch")

// Secret is a revealed resource credential; it is never logged or stored.
type Secret struct {
	Kind  string
	Value []byte
}

// LogValue keeps the credential out of logs.
func (Secret) LogValue() slog.Value { return slog.StringValue("[redacted]") }

// Each unary call runs under callTimeout.

// AuthenticateUser resolves the user of an SSH key fingerprint.
func (c *tunnelClient) AuthenticateUser(ctx context.Context, fingerprint string, ip netip.Addr) (User, error) {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	resp, err := c.conn.Load().rpc.AuthenticateUser(ctx, connect.NewRequest(&gatewayv1.AuthenticateUserRequest{
		By: &gatewayv1.AuthenticateUserRequest_FingerprintSha256{FingerprintSha256: fingerprint}, ClientIp: ip.String(),
	}))
	if err != nil {
		return User{}, err
	}
	m := resp.Msg
	u := User{Login: m.GetLogin(), Country: m.GetCountry()}
	if u.ID, err = uuid.Parse(m.GetUserId()); err != nil {
		return User{}, errors.New("authenticate user: invalid user_id in response")
	}
	for _, s := range m.GetTeamIds() {
		id, err := uuid.Parse(s)
		if err != nil {
			return User{}, errors.New("authenticate user: invalid team_id in response")
		}
		u.TeamIDs = append(u.TeamIDs, id)
	}
	return u, nil
}

// StartSession records a session of kind (target: host:port for a stream)
// and reveals the resource credential.
func (c *tunnelClient) StartSession(ctx context.Context, userID, resourceID uuid.UUID, ip netip.Addr, kind Kind, target string) (uuid.UUID, Secret, error) {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	resp, err := c.conn.Load().rpc.StartSession(ctx, connect.NewRequest(&gatewayv1.StartSessionRequest{
		UserId: userID.String(), ResourceId: resourceID.String(), ClientIp: ip.String(), Kind: string(kind), Target: target,
	}))
	if err != nil {
		return uuid.Nil, Secret{}, err
	}
	id, err := uuid.Parse(resp.Msg.GetSessionId())
	if err != nil {
		return uuid.Nil, Secret{}, errors.New("start session: invalid session_id in response")
	}
	cred := resp.Msg.GetCredential()
	if cred == nil {
		return uuid.Nil, Secret{}, errors.New("start session: no credential in response")
	}
	return id, Secret{Kind: cred.GetKind(), Value: cred.GetValue()}, nil
}

// EndSession closes a session with a gateway-side reason.
func (c *tunnelClient) EndSession(ctx context.Context, sessionID uuid.UUID, reason string) error {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	_, err := c.conn.Load().rpc.EndSession(ctx, connect.NewRequest(&gatewayv1.EndSessionRequest{SessionId: sessionID.String(), Reason: reason}))
	return err
}

// PinHostKey pins the target host key on first use; a different pinned key is ErrHostKeyMismatch.
func (c *tunnelClient) PinHostKey(ctx context.Context, resourceID uuid.UUID, alg string, key []byte) error {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	_, err := c.conn.Load().rpc.PinHostKey(ctx, connect.NewRequest(&gatewayv1.PinHostKeyRequest{
		ResourceId: resourceID.String(), Alg: alg, PublicKey: key,
	}))
	if connect.CodeOf(err) == connect.CodeFailedPrecondition {
		return fmt.Errorf("%w: %w", ErrHostKeyMismatch, err)
	}
	return err
}

// UploadChunk sends one spooled recording chunk.
func (c *tunnelClient) UploadChunk(ctx context.Context, sessionID uuid.UUID, seq int, hash, gz []byte) error {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	_, err := c.conn.Load().rpc.UploadRecordingChunk(ctx, connect.NewRequest(&gatewayv1.UploadRecordingChunkRequest{
		SessionId: sessionID.String(), Seq: int32(seq), Hash: hash, Gz: gz, //nolint:gosec // seq is bounded by the session's chunk count
	}))
	return err
}

// FinishRecording closes a recording with the totals of its chunks.
func (c *tunnelClient) FinishRecording(ctx context.Context, m recorder.Manifest) error {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	_, err := c.conn.Load().rpc.FinishRecording(ctx, connect.NewRequest(&gatewayv1.FinishRecordingRequest{
		SessionId: m.SessionID.String(), Chunks: int32(m.Chunks), Bytes: m.Bytes, FinalHash: m.FinalHash, //nolint:gosec // chunks is bounded like seq
	}))
	return err
}
