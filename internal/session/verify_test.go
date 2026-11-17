package session_test

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/session"
	"github.com/werbot/werbot/internal/session/recstore"
)

// finishedEnv is a recEnv over a directory store with 3 ingested chunks, finished.
func finishedEnv(t *testing.T) (e recEnv, dir string) {
	t.Helper()
	dir = t.TempDir()
	e = newRecEnv(t)
	e.store.RecordingStore = recstore.NewFS(dir)
	gz, hashes := makeChunks(t, 3)
	e.ingestAll(t, gz, hashes)
	require.NoError(t, e.finish(t, 3, sumLen(gz), hashes[2]))
	return e, dir
}

func (e recEnv) verify(t *testing.T) session.Verification {
	t.Helper()
	v, err := e.rec.Verify(t.Context(), e.f.OrgID, e.sid)
	require.NoError(t, err)
	return v
}

func (e recEnv) path(dir string, seq int) string {
	return filepath.Join(dir, recstore.Key(e.f.OrgID, e.sid, seq))
}

func TestVerifyOK(t *testing.T) {
	e, _ := finishedEnv(t)
	assert.Equal(t, session.Verification{Verified: true}, e.verify(t))
}

func TestVerifyModifiedChunk(t *testing.T) {
	e, dir := finishedEnv(t)
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	_, err := w.Write([]byte("forged"))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, os.WriteFile(e.path(dir, 1), b.Bytes(), 0o600))
	assert.Equal(t, session.Verification{Reason: session.ReasonChainMismatch}, e.verify(t))
}

func TestVerifyDeletedChunk(t *testing.T) {
	e, dir := finishedEnv(t)
	require.NoError(t, os.Remove(e.path(dir, 2)))
	assert.Equal(t, session.Verification{Reason: session.ReasonChunkMissing}, e.verify(t))
}

func TestVerifyReorderedChunks(t *testing.T) {
	e, dir := finishedEnv(t)
	a, b := e.path(dir, 0), e.path(dir, 1)
	tmp := a + ".tmp"
	require.NoError(t, os.Rename(a, tmp))
	require.NoError(t, os.Rename(b, a))
	require.NoError(t, os.Rename(tmp, b))
	assert.Equal(t, session.Verification{Reason: session.ReasonChainMismatch}, e.verify(t))
}

func TestVerifyTamperedRow(t *testing.T) {
	e, _ := finishedEnv(t)
	_, err := e.pool.Exec(t.Context(), "UPDATE recording SET bytes = bytes + 1 WHERE session_id = $1", e.sid)
	require.NoError(t, err)
	assert.Equal(t, session.Verification{Reason: session.ReasonSignatureInvalid}, e.verify(t))
}

func TestVerifyNotFinished(t *testing.T) {
	e := newRecEnv(t)
	gz, hashes := makeChunks(t, 2)
	e.ingestAll(t, gz, hashes)
	assert.Equal(t, session.Verification{Reason: session.ReasonNotFinished}, e.verify(t))
}

func TestVerifyNoRecording(t *testing.T) {
	e := newRecEnv(t)
	assert.Equal(t, session.Verification{Reason: session.ReasonNoRecording}, e.verify(t))
}

func TestVerifyOtherOrg(t *testing.T) {
	e, _ := finishedEnv(t)
	other := dbtest.Seed(t, e.pool)
	v, err := e.rec.Verify(t.Context(), other.OrgID, e.sid)
	require.NoError(t, err)
	assert.Equal(t, session.Verification{Reason: session.ReasonNoRecording}, v)
}

func TestVerifyStoreUnavailable(t *testing.T) {
	e, _ := finishedEnv(t)
	e.store.down.Store(true)
	_, err := e.rec.Verify(t.Context(), e.f.OrgID, e.sid)
	require.ErrorIs(t, err, apperr.ErrUnavailable)
}
