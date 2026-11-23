package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/gateway/recorder"
	"github.com/werbot/werbot/internal/platform/shellclass"
	"github.com/werbot/werbot/internal/platform/sshca"
)

// ErrHostKeyMismatch is a target host key that differs from the pinned one.
var ErrHostKeyMismatch = errors.New("SSH host key mismatch")

// ErrLoginFailed is a refused or timed-out automatic login to the target (telnet).
var ErrLoginFailed = errors.New("login to the target failed")

// Secret is a revealed resource credential; it is never logged or stored.
type Secret struct {
	Kind  string
	Value []byte
}

// LogValue keeps the credential out of logs.
func (Secret) LogValue() slog.Value { return slog.StringValue("[redacted]") }

// Each unary call runs under callTimeout.

// AuthenticateUser resolves the user of an SSH key fingerprint or of a
// verified certificate, with the agent run of an agent certificate.
func (c *tunnelClient) AuthenticateUser(ctx context.Context, id Identity, ip netip.Addr) (User, error) {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	resp, err := c.conn.Load().rpc.AuthenticateUser(ctx, connect.NewRequest(authRequest(id, ip)))
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
	return withAgent(u, id.Cert, m.GetAgent())
}

// errAgentMismatch is an AuthenticateUser answer whose agent run is not the
// one of the certificate: the user is refused, never taken as a person.
var errAgentMismatch = errors.New("authenticate user: agent does not match the certificate")

// withAgent is u with the agent run a of the answer for cert: the run of an
// agent certificate must come back as it is, sponsored by u, and only for one.
func withAgent(u User, cert *sshca.Identity, a *gatewayv1.AgentInfo) (User, error) {
	isAgent := cert != nil && cert.AgentID != nil
	switch {
	case !isAgent && a == nil:
		return u, nil
	case !isAgent || a == nil, a.GetAgentId() != cert.AgentID.String(), a.GetRunId() != cert.RunID.String(),
		a.GetSponsorUserId() != u.ID.String():
		return User{}, errAgentMismatch
	}
	u.Agent = &Agent{ID: *cert.AgentID, RunID: *cert.RunID, AuthAt: time.Now()}
	return u, nil
}

// authRequest asks for the user of id: by the certificate's werbot identity,
// else by the key fingerprint.
func authRequest(id Identity, ip netip.Addr) *gatewayv1.AuthenticateUserRequest {
	r := &gatewayv1.AuthenticateUserRequest{ClientIp: ip.String(),
		By: &gatewayv1.AuthenticateUserRequest_FingerprintSha256{FingerprintSha256: id.Value}}
	if c := id.Cert; c != nil {
		r.By = &gatewayv1.AuthenticateUserRequest_Cert{Cert: &gatewayv1.CertIdentity{
			UserId: c.UserID.String(), CliSessionId: c.CLISessionID.String(), Login: c.Login, CertFingerprint: id.Value,
			AgentId: optionalID(c.AgentID), RunId: optionalID(c.RunID)}}
	}
	return r
}

// StartSession records a session of kind (target: host:port for a stream; an
// exec: its authorized command, cut to the wire bound, and classes; an
// agent's: its run and tool call) and reveals the resource credential.
func (c *tunnelClient) StartSession(ctx context.Context, userID, resourceID uuid.UUID, ip netip.Addr, kind Kind, target, command string, classes, paths []string, agent agentStart) (uuid.UUID, Secret, error) {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	resp, err := c.conn.Load().rpc.StartSession(ctx, connect.NewRequest(&gatewayv1.StartSessionRequest{
		UserId: userID.String(), ResourceId: resourceID.String(), ClientIp: ip.String(), Kind: string(kind), Target: target,
		Command: TruncateUTF8(command, 65536), Classes: classes, Paths: paths[:min(len(paths), shellclass.MaxPaths)],
		AgentRunId: idString(agent.RunID), ToolCallId: idString(agent.ToolCall),
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

// EndSession closes a session with a gateway-side reason, its final counters
// and its console or exec result (nil: none); the counters get the session id
// the server requires, the result is cut to the wire bounds.
func (c *tunnelClient) EndSession(ctx context.Context, sessionID uuid.UUID, reason string, counters *gatewayv1.SessionCounters, r *recorder.SessionResult) error {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	if counters != nil {
		counters.SessionId = sessionID.String()
	}
	req := &gatewayv1.EndSessionRequest{SessionId: sessionID.String(), Reason: reason, Counters: counters}
	if r != nil {
		req.Command, req.Classes = TruncateUTF8(r.Command, 65536), r.Classes
		req.Paths = r.Paths[:min(len(r.Paths), shellclass.MaxPaths)]
		req.ExitCode, req.OutputSha256 = r.ExitCode, r.OutputSHA256
	}
	_, err := c.conn.Load().rpc.EndSession(ctx, connect.NewRequest(req))
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

// RequestApproval asks the approvers to allow a command; the decision comes as
// a hint on the stream or from GetApproval.
func (c *tunnelClient) RequestApproval(ctx context.Context, r approvalRequest) (approvalTicket, error) {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	resp, err := c.conn.Load().rpc.RequestApproval(ctx, connect.NewRequest(&gatewayv1.RequestApprovalRequest{
		UserId: r.UserID.String(), ResourceId: r.ResourceID.String(), AgentRunId: idString(r.AgentRunID), Command: r.Command,
		Classes: r.Classes, Reasons: r.Reasons,
	}))
	if err != nil {
		return approvalTicket{}, err
	}
	m := resp.Msg
	return approvalTicket{ID: m.GetApprovalId(), URL: m.GetUrl(), Status: m.GetStatus(), ExpiresAt: m.GetExpiresAt().AsTime()}, nil
}

// RecordDenial reports a policy denial of agent run runID at once; the
// server counts it toward the run's pause and answers whether it paused.
func (c *tunnelClient) RecordDenial(ctx context.Context, runID uuid.UUID, resource, command string, classes, reasons []string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	resp, err := c.conn.Load().rpc.RecordDenial(ctx, connect.NewRequest(&gatewayv1.RecordDenialRequest{
		RunId: runID.String(), Resource: resource, Command: command, Classes: classes, Reasons: reasons,
	}))
	if err != nil {
		return false, err
	}
	return resp.Msg.GetPaused(), nil
}

// GetApproval returns the status of approval id: pending, approved, denied, expired or canceled.
func (c *tunnelClient) GetApproval(ctx context.Context, id string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	resp, err := c.conn.Load().rpc.GetApproval(ctx, connect.NewRequest(&gatewayv1.GetApprovalRequest{ApprovalId: id}))
	if err != nil {
		return "", err
	}
	return resp.Msg.GetStatus(), nil
}
