package session_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/session"
)

func (e recEnv) signature(t *testing.T) []byte {
	t.Helper()
	var sig []byte
	require.NoError(t, e.pool.QueryRow(t.Context(),
		"SELECT manifest_signature FROM recording WHERE session_id = $1", e.sid).Scan(&sig))
	return sig
}

func (e recEnv) finishEvents(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, e.pool.QueryRow(t.Context(),
		"SELECT count(*) FROM audit_event WHERE action = 'recording.finish' AND target_id = $1", e.sid.String()).Scan(&n))
	return n
}

func TestIngestAndFinish(t *testing.T) {
	e := newRecEnv(t)
	gz, hashes := makeChunks(t, 3)
	e.ingestAll(t, gz, hashes)

	chunks, size, last := e.row(t, e.sid)
	assert.Equal(t, 3, chunks)
	assert.Equal(t, sumLen(gz), size)
	assert.Equal(t, hashes[2], last)
	for i := range gz {
		got, err := e.object(t, i)
		require.NoError(t, err)
		assert.Equal(t, gz[i], got)
	}

	require.NoError(t, e.finish(t, 3, sumLen(gz), hashes[2]))
	var final, sig []byte
	var verified bool
	require.NoError(t, e.pool.QueryRow(t.Context(),
		"SELECT final_hash, manifest_signature, verified_at IS NOT NULL FROM recording WHERE session_id = $1", e.sid).
		Scan(&final, &sig, &verified))
	assert.Equal(t, hashes[2], final)
	assert.True(t, verified)
	assert.True(t, e.signer.Verify(session.NewManifest(e.sid, 3, sumLen(gz), hashes[2]).Canonical(), sig))

	var kind, target string
	var raw []byte
	require.NoError(t, e.pool.QueryRow(t.Context(),
		"SELECT actor_kind, target_type, data FROM audit_event WHERE action = 'recording.finish'").Scan(&kind, &target, &raw))
	assert.Equal(t, "gateway", kind)
	assert.Equal(t, "session", target)
	var data map[string]any
	require.NoError(t, json.Unmarshal(raw, &data))
	assert.Equal(t, map[string]any{
		"gateway_instance_id": e.f.InstanceID.String(), "chunks": float64(3), "bytes": float64(sumLen(gz)),
	}, data)
	assert.Equal(t, 1, e.finishEvents(t))
}

func TestFinishMismatch(t *testing.T) {
	e := newRecEnv(t)
	gz, hashes := makeChunks(t, 3)
	require.ErrorIs(t, e.finish(t, 0, 0, hashes[0]), apperr.ErrFailedPrecondition) // no recording yet
	e.ingestAll(t, gz, hashes)

	require.ErrorIs(t, e.finish(t, 3, sumLen(gz), hashes[1]), apperr.ErrFailedPrecondition)
	require.ErrorIs(t, e.finish(t, 2, sumLen(gz), hashes[2]), apperr.ErrFailedPrecondition)
	require.ErrorIs(t, e.finish(t, 3, sumLen(gz)+1, hashes[2]), apperr.ErrFailedPrecondition)
	assert.Nil(t, e.signature(t))
	assert.Equal(t, 0, e.finishEvents(t))
}

func TestFinishIdempotent(t *testing.T) {
	e := newRecEnv(t)
	gz, hashes := makeChunks(t, 3)
	e.ingestAll(t, gz, hashes)
	require.NoError(t, e.finish(t, 3, sumLen(gz), hashes[2]))
	require.NoError(t, e.finish(t, 3, sumLen(gz), hashes[2]))
	require.ErrorIs(t, e.finish(t, 3, sumLen(gz), hashes[1]), apperr.ErrFailedPrecondition)
	assert.Equal(t, 1, e.finishEvents(t))
}

func TestFinishOtherInstance(t *testing.T) {
	e := newRecEnv(t)
	gz, hashes := makeChunks(t, 3)
	e.ingestAll(t, gz, hashes)
	err := e.rec.FinishRecording(t.Context(), e.f.OrgID, e.f.OtherInstanceID, e.sid, 3, sumLen(gz), hashes[2])
	require.ErrorIs(t, err, apperr.ErrPermissionDenied)
	assert.Nil(t, e.signature(t))
}

func TestIngestAfterFinish(t *testing.T) {
	e := newRecEnv(t)
	gz, hashes := makeChunks(t, 4)
	e.ingestAll(t, gz[:3], hashes[:3])
	require.NoError(t, e.finish(t, 3, sumLen(gz[:3]), hashes[2]))

	require.ErrorIs(t, e.ingest(t, 3, hashes[3], gz[3]), apperr.ErrFailedPrecondition)
	require.ErrorIs(t, e.ingest(t, 2, hashes[2], gz[2]), apperr.ErrAlreadyExists)
}

func TestIngestRetryAfterPutCommittedTxRolledBack(t *testing.T) {
	e := newRecEnv(t)
	gz, hashes := makeChunks(t, 3)
	e.ingestAll(t, gz[:1], hashes[:1])

	e.store.lost.Store(true)
	require.ErrorIs(t, e.ingest(t, 1, hashes[1], gz[1]), apperr.ErrUnavailable)
	chunks, size, last := e.row(t, e.sid)
	assert.Equal(t, 1, chunks)
	assert.Equal(t, int64(len(gz[0])), size)
	assert.Equal(t, hashes[0], last)
	got, err := e.object(t, 1)
	require.NoError(t, err)
	assert.Equal(t, gz[1], got, "Put ran before the tx rolled back")

	// A torn leftover object must be replaced by the retry.
	e.store.lost.Store(false)
	require.NoError(t, e.store.RecordingStore.Put(t.Context(), e.f.OrgID, e.sid, 1, gz[1][:3]))
	require.NoError(t, e.ingest(t, 1, hashes[1], gz[1]))
	got, err = e.object(t, 1)
	require.NoError(t, err)
	assert.Equal(t, gz[1], got)

	require.NoError(t, e.ingest(t, 2, hashes[2], gz[2]))
	require.NoError(t, e.finish(t, 3, sumLen(gz), hashes[2]))
	assert.True(t, e.signer.Verify(session.NewManifest(e.sid, 3, sumLen(gz), hashes[2]).Canonical(), e.signature(t)))
}
