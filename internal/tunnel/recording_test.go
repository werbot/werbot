package tunnel

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
)

func chunkReq(sessionID string) *gatewayv1.UploadRecordingChunkRequest {
	return &gatewayv1.UploadRecordingChunkRequest{SessionId: sessionID, Seq: 3,
		Hash: bytes.Repeat([]byte{7}, 32), Gz: []byte("gzip bytes")}
}

func TestUploadRecordingChunkUsesGatewayIdentity(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	sid := uuid.New()
	req := chunkReq(sid.String())

	_, err := g.client.UploadRecordingChunk(ctx, connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	b.f.ingest.with(func(r *fakeIngest) {
		want := recCall{orgID: b.org, instanceID: g.InstanceID, sessionID: sid, n: 3}
		if len(r.chunks) != 1 {
			t.Fatalf("IngestChunk calls = %d, want 1", len(r.chunks))
		}
		got := r.chunks[0]
		if got.orgID != want.orgID || got.instanceID != want.instanceID || got.sessionID != want.sessionID || got.n != want.n ||
			!bytes.Equal(got.hash, req.GetHash()) || !bytes.Equal(got.gz, req.GetGz()) {
			t.Fatalf("IngestChunk(%+v), want org and instance from the certificate, %+v", got, want)
		}
	})
}

func TestUploadRecordingChunkMapsErrors(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	for _, tc := range []struct {
		err  error
		code connect.Code
	}{
		{fmt.Errorf("x: %w", apperr.ErrAlreadyExists), connect.CodeAlreadyExists},
		{apperr.ErrPermissionDenied, connect.CodePermissionDenied},
		{fmt.Errorf("recording store: %w: %w", apperr.ErrUnavailable, io.ErrUnexpectedEOF), connect.CodeUnavailable},
	} {
		b.f.ingest.with(func(r *fakeIngest) { r.err = tc.err })
		_, err := g.client.UploadRecordingChunk(ctx, connect.NewRequest(chunkReq(uuid.NewString())))
		wantCode(t, err, tc.code, "")
	}
}

func TestUploadRecordingChunkBadSessionID(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	_, err := g.client.UploadRecordingChunk(ctx, connect.NewRequest(chunkReq("nope")))
	wantCode(t, err, connect.CodeInvalidArgument, "")
	b.f.ingest.with(func(r *fakeIngest) {
		if len(r.chunks) != 0 {
			t.Fatal("IngestChunk called for an invalid session_id")
		}
	})
}

func TestUploadRecordingChunkValidates(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	for name, tweak := range map[string]func(r *gatewayv1.UploadRecordingChunkRequest){
		"negative seq": func(r *gatewayv1.UploadRecordingChunkRequest) { r.Seq = -1 },
		"short hash":   func(r *gatewayv1.UploadRecordingChunkRequest) { r.Hash = r.Hash[:31] },
		"empty gz":     func(r *gatewayv1.UploadRecordingChunkRequest) { r.Gz = nil },
		"large gz":     func(r *gatewayv1.UploadRecordingChunkRequest) { r.Gz = make([]byte, 128<<10+1) },
	} {
		req := chunkReq(uuid.NewString())
		tweak(req)
		_, err := g.client.UploadRecordingChunk(ctx, connect.NewRequest(req))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("%s: err = %v, want InvalidArgument", name, err)
		}
	}
	b.f.ingest.with(func(r *fakeIngest) {
		if len(r.chunks) != 0 {
			t.Fatal("IngestChunk called for an invalid request")
		}
	})
}

func TestFinishRecordingPassesManifest(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	sid, finalHash := uuid.New(), bytes.Repeat([]byte{9}, 32)

	_, err := g.client.FinishRecording(ctx, connect.NewRequest(&gatewayv1.FinishRecordingRequest{
		SessionId: sid.String(), Chunks: 3, Bytes: 42, FinalHash: finalHash}))
	if err != nil {
		t.Fatal(err)
	}
	b.f.ingest.with(func(r *fakeIngest) {
		if len(r.finishes) != 1 {
			t.Fatalf("FinishRecording calls = %d, want 1", len(r.finishes))
		}
		got := r.finishes[0]
		if got.orgID != b.org || got.instanceID != g.InstanceID || got.sessionID != sid || got.n != 3 || got.bytes != 42 ||
			!bytes.Equal(got.hash, finalHash) {
			t.Fatalf("FinishRecording(%+v), want (org, inst, %v, 3, 42, finalHash)", got, sid)
		}
	})

	b.f.ingest.with(func(r *fakeIngest) { r.err = fmt.Errorf("mismatch: %w", apperr.ErrFailedPrecondition) })
	_, err = g.client.FinishRecording(ctx, connect.NewRequest(&gatewayv1.FinishRecordingRequest{
		SessionId: sid.String(), Chunks: 3, Bytes: 42, FinalHash: finalHash}))
	wantCode(t, err, connect.CodeFailedPrecondition, "")

	_, err = g.client.FinishRecording(ctx, connect.NewRequest(&gatewayv1.FinishRecordingRequest{
		SessionId: sid.String(), Chunks: 0, Bytes: 42, FinalHash: finalHash}))
	wantCode(t, err, connect.CodeInvalidArgument, "")
	b.f.ingest.with(func(r *fakeIngest) {
		if len(r.finishes) != 2 {
			t.Fatalf("FinishRecording calls = %d, want 2 (invalid request rejected first)", len(r.finishes))
		}
	})
}
