package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/gateway/recorder"
)

// recCalls is what the fake TunnelService got through the recording RPCs; f.mu guards it.
type recCalls struct {
	chunks   []*gatewayv1.UploadRecordingChunkRequest
	finishes []*gatewayv1.FinishRecordingRequest
	order    []string // "end", "chunk", "finish": the spool RPCs in call order
	err      error    // returned by both recording RPCs when set
}

func (f *fakeTunnel) EndSession(_ context.Context, r *connect.Request[gatewayv1.EndSessionRequest]) (*connect.Response[gatewayv1.EndSessionResponse], error) {
	f.mu.Lock()
	f.ends = append(f.ends, r.Msg)
	f.rec.order = append(f.rec.order, "end")
	f.mu.Unlock()
	return connect.NewResponse(&gatewayv1.EndSessionResponse{}), nil
}

func (f *fakeTunnel) UploadRecordingChunk(_ context.Context, r *connect.Request[gatewayv1.UploadRecordingChunkRequest]) (*connect.Response[gatewayv1.UploadRecordingChunkResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rec.chunks = append(f.rec.chunks, r.Msg)
	f.rec.order = append(f.rec.order, "chunk")
	if f.rec.err != nil {
		return nil, f.rec.err
	}
	return connect.NewResponse(&gatewayv1.UploadRecordingChunkResponse{}), nil
}

func (f *fakeTunnel) FinishRecording(_ context.Context, r *connect.Request[gatewayv1.FinishRecordingRequest]) (*connect.Response[gatewayv1.FinishRecordingResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rec.finishes = append(f.rec.finishes, r.Msg)
	f.rec.order = append(f.rec.order, "finish")
	if f.rec.err != nil {
		return nil, f.rec.err
	}
	return connect.NewResponse(&gatewayv1.FinishRecordingResponse{}), nil
}

func TestTunnelClientUploadsRecording(t *testing.T) {
	e := newTunnelEnv(t, nil, time.Hour)
	hash, gz := make([]byte, 32), []byte("gz-bytes")
	hash[0] = 7
	require.NoError(t, e.c.UploadChunk(t.Context(), testSessionID, 3, hash, gz))
	m := recorder.Manifest{SessionID: testSessionID, Chunks: 4, Bytes: 99, FinalHash: hash}
	require.NoError(t, e.c.FinishRecording(t.Context(), m))
	e.f.with(func(f *fakeTunnel) {
		c := f.rec.chunks[0]
		require.Equal(t, testSessionID.String(), c.GetSessionId())
		require.EqualValues(t, 3, c.GetSeq())
		require.Equal(t, hash, c.GetHash())
		require.Equal(t, gz, c.GetGz())
		fin := f.rec.finishes[0]
		require.Equal(t, testSessionID.String(), fin.GetSessionId())
		require.EqualValues(t, 4, fin.GetChunks())
		require.EqualValues(t, 99, fin.GetBytes())
		require.Equal(t, hash, fin.GetFinalHash())
	})

	for _, code := range []connect.Code{connect.CodeAlreadyExists, connect.CodeUnavailable} {
		e.f.with(func(f *fakeTunnel) { f.rec.err = connect.NewError(code, errors.New("answer")) })
		require.Equal(t, code, connect.CodeOf(e.c.UploadChunk(t.Context(), testSessionID, 0, hash, gz)))
		require.Equal(t, code, connect.CodeOf(e.c.FinishRecording(t.Context(), m)))
	}
}
