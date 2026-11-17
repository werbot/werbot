package tunnel

import (
	"context"

	"connectrpc.com/connect"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// UploadRecordingChunk stores one recording chunk of a session served by the
// calling instance; org and instance come from the certificate.
func (h *Handler) UploadRecordingChunk(ctx context.Context, req *connect.Request[gatewayv1.UploadRecordingChunkRequest]) (*connect.Response[gatewayv1.UploadRecordingChunkResponse], error) {
	g, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetSessionId(), "session_id")
	if err != nil {
		return nil, err
	}
	m := req.Msg
	if err := h.d.Recordings.IngestChunk(ctx, g.OrgID, g.InstanceID, id, int(m.GetSeq()), m.GetHash(), m.GetGz()); err != nil {
		return nil, h.fail(ctx, "upload recording chunk", err)
	}
	return connect.NewResponse(&gatewayv1.UploadRecordingChunkResponse{}), nil
}

// FinishRecording closes the recording of a session served by the calling instance.
func (h *Handler) FinishRecording(ctx context.Context, req *connect.Request[gatewayv1.FinishRecordingRequest]) (*connect.Response[gatewayv1.FinishRecordingResponse], error) {
	g, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetSessionId(), "session_id")
	if err != nil {
		return nil, err
	}
	m := req.Msg
	if err := h.d.Recordings.FinishRecording(ctx, g.OrgID, g.InstanceID, id, int(m.GetChunks()), m.GetBytes(), m.GetFinalHash()); err != nil {
		return nil, h.fail(ctx, "finish recording", err)
	}
	return connect.NewResponse(&gatewayv1.FinishRecordingResponse{}), nil
}
