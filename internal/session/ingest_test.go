package session_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/hashchain"
	"github.com/werbot/werbot/internal/platform/secretbox"
	"github.com/werbot/werbot/internal/session"
	"github.com/werbot/werbot/internal/session/recstore"
)

// makeChunks gzips "chunk-<i>" for i in [0, n) and chains their hashes from Genesis.
func makeChunks(t testing.TB, n int) (gz, hashes [][]byte) {
	t.Helper()
	prev := hashchain.Genesis()
	for i := range n {
		var b bytes.Buffer
		w := gzip.NewWriter(&b)
		_, err := fmt.Fprintf(w, "chunk-%d", i)
		require.NoError(t, err)
		require.NoError(t, w.Close())
		prev = hashchain.Next(prev, b.Bytes())
		gz, hashes = append(gz, b.Bytes()), append(hashes, prev)
	}
	return gz, hashes
}

func sumLen(gz [][]byte) (n int64) {
	for _, g := range gz {
		n += int64(len(g))
	}
	return n
}

// flakyStore fails Put and Get while down is set.
type flakyStore struct {
	session.RecordingStore
	down atomic.Bool
}

func (s *flakyStore) Put(ctx context.Context, orgID, sessionID uuid.UUID, seq int, gz []byte) error {
	if s.down.Load() {
		return errors.New("store down")
	}
	return s.RecordingStore.Put(ctx, orgID, sessionID, seq, gz)
}

func (s *flakyStore) Get(ctx context.Context, orgID, sessionID uuid.UUID, seq int) (io.ReadCloser, error) {
	if s.down.Load() {
		return nil, errors.New("store down")
	}
	return s.RecordingStore.Get(ctx, orgID, sessionID, seq)
}

// recEnv is one seeded organization with an open session of f.MemberID on f.InstanceID.
type recEnv struct {
	pool   *pgxpool.Pool
	f      dbtest.Fixture
	signer *session.Signer
	store  *flakyStore
	rec    *session.Recordings
	sid    uuid.UUID
}

func newRecEnv(t *testing.T) recEnv {
	t.Helper()
	pool, _ := dbtest.New(t)
	f := dbtest.Seed(t, pool)
	box, err := secretbox.New(key(t), nil)
	require.NoError(t, err)
	signer, err := session.LoadSigner(t.Context(), pool, box)
	require.NoError(t, err)
	store := &flakyStore{RecordingStore: recstore.NewFS(t.TempDir())}
	return recEnv{pool: pool, f: f, signer: signer, store: store,
		rec: session.NewRecordings(pool, store, "fs", signer), sid: f.StartSession(t, pool, f.MemberID)}
}

func (e recEnv) ingest(t *testing.T, seq int, hash, gz []byte) error {
	t.Helper()
	return e.rec.IngestChunk(t.Context(), e.f.OrgID, e.f.InstanceID, e.sid, seq, hash, gz)
}

func (e recEnv) ingestAll(t *testing.T, gz, hashes [][]byte) {
	t.Helper()
	for i := range gz {
		require.NoError(t, e.ingest(t, i, hashes[i], gz[i]))
	}
}

func (e recEnv) finish(t *testing.T, chunks int, size int64, finalHash []byte) error {
	t.Helper()
	return e.rec.FinishRecording(t.Context(), e.f.OrgID, e.f.InstanceID, e.sid, chunks, size, finalHash)
}

// row returns the recording row of sid.
func (e recEnv) row(t *testing.T, sid uuid.UUID) (chunks int, size int64, lastHash []byte) {
	t.Helper()
	require.NoError(t, e.pool.QueryRow(t.Context(),
		"SELECT chunks, bytes, last_hash FROM recording WHERE session_id = $1", sid).Scan(&chunks, &size, &lastHash))
	return chunks, size, lastHash
}

func (e recEnv) hasRecording(t *testing.T) bool {
	t.Helper()
	var ok bool
	require.NoError(t, e.pool.QueryRow(t.Context(),
		"SELECT EXISTS (SELECT 1 FROM recording WHERE session_id = $1)", e.sid).Scan(&ok))
	return ok
}

// object reads chunk seq of sid from the store.
func (e recEnv) object(t *testing.T, seq int) ([]byte, error) {
	t.Helper()
	rc, err := e.store.Get(t.Context(), e.f.OrgID, e.sid, seq)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func TestRecordingDefaults(t *testing.T) {
	e := newRecEnv(t)
	ctx := t.Context()
	_, err := e.pool.Exec(ctx, "INSERT INTO recording (session_id, org_id, store) VALUES ($1, $2, 'fs')", e.sid, e.f.OrgID)
	require.NoError(t, err)
	chunks, size, last := e.row(t, e.sid)
	assert.Equal(t, 0, chunks)
	assert.Equal(t, int64(0), size)
	assert.Equal(t, hashchain.Genesis(), last)

	_, err = e.pool.Exec(ctx, `UPDATE recording SET last_hash = '\x00' WHERE session_id = $1`, e.sid)
	require.ErrorContains(t, err, "check constraint")

	var absent bool
	require.NoError(t, e.pool.QueryRow(ctx, "SELECT to_regclass('recording_chunk') IS NULL").Scan(&absent))
	assert.True(t, absent)
}

func TestIngestDuplicateSeq(t *testing.T) {
	e := newRecEnv(t)
	gz, hashes := makeChunks(t, 2)
	require.NoError(t, e.ingest(t, 0, hashes[0], gz[0]))

	other := hashchain.Next(hashchain.Genesis(), gz[1])
	require.ErrorIs(t, e.ingest(t, 0, other, gz[1]), apperr.ErrAlreadyExists)
	got, err := e.object(t, 0)
	require.NoError(t, err)
	assert.Equal(t, gz[0], got)
	_, _, last := e.row(t, e.sid)
	assert.Equal(t, hashes[0], last)
}

func TestIngestConcurrentSameSeq(t *testing.T) {
	e := newRecEnv(t)
	recs := []*session.Recordings{e.rec, session.NewRecordings(e.pool, e.store, "fs", e.signer)}
	gz, hashes := makeChunks(t, 1)
	for range 20 {
		sid := e.f.StartSession(t, e.pool, e.f.MemberID)
		start := make(chan struct{})
		errs := make([]error, len(recs))
		var wg sync.WaitGroup
		for i, r := range recs {
			wg.Go(func() {
				<-start
				errs[i] = r.IngestChunk(t.Context(), e.f.OrgID, e.f.InstanceID, sid, 0, hashes[0], gz[0])
			})
		}
		close(start)
		wg.Wait()

		ok, dup := 0, 0
		for _, err := range errs {
			switch {
			case err == nil:
				ok++
			case errors.Is(err, apperr.ErrAlreadyExists):
				dup++
			default:
				t.Fatalf("unexpected error: %v", err)
			}
		}
		assert.Equal(t, 1, ok)
		assert.Equal(t, 1, dup)
		chunks, size, last := e.row(t, sid)
		assert.Equal(t, 1, chunks)
		assert.Equal(t, int64(len(gz[0])), size)
		assert.Equal(t, hashes[0], last)
	}
}

func TestIngestOutOfOrder(t *testing.T) {
	e := newRecEnv(t)
	gz, hashes := makeChunks(t, 2)
	require.ErrorIs(t, e.ingest(t, 1, hashes[1], gz[1]), apperr.ErrFailedPrecondition)
	require.ErrorIs(t, e.ingest(t, -1, hashes[0], gz[0]), apperr.ErrInvalidArgument)
	require.NoError(t, e.ingest(t, 0, hashes[0], gz[0]))
}

func TestIngestWrongHash(t *testing.T) {
	e := newRecEnv(t)
	gz, hashes := makeChunks(t, 2)
	require.NoError(t, e.ingest(t, 0, hashes[0], gz[0]))
	require.ErrorIs(t, e.ingest(t, 1, hashchain.Next(hashchain.Genesis(), gz[1]), gz[1]), apperr.ErrInvalidArgument)

	chunks, _, last := e.row(t, e.sid)
	assert.Equal(t, 1, chunks)
	assert.Equal(t, hashes[0], last)
	_, err := e.object(t, 1)
	require.ErrorIs(t, err, fs.ErrNotExist)
}

func TestIngestStoreUnavailable(t *testing.T) {
	e := newRecEnv(t)
	gz, hashes := makeChunks(t, 1)
	e.store.down.Store(true)
	require.ErrorIs(t, e.ingest(t, 0, hashes[0], gz[0]), apperr.ErrUnavailable)
	assert.False(t, e.hasRecording(t))

	e.store.down.Store(false)
	require.NoError(t, e.ingest(t, 0, hashes[0], gz[0]))
	chunks, _, last := e.row(t, e.sid)
	assert.Equal(t, 1, chunks)
	assert.Equal(t, hashes[0], last)
}

func TestIngestOtherInstance(t *testing.T) {
	e := newRecEnv(t)
	gz, hashes := makeChunks(t, 1)
	sibling := dbtest.Instance(t, e.pool, e.f.OrgID, e.f.GatewayID)
	for _, inst := range []uuid.UUID{e.f.OtherInstanceID, sibling} {
		err := e.rec.IngestChunk(t.Context(), e.f.OrgID, inst, e.sid, 0, hashes[0], gz[0])
		require.ErrorIs(t, err, apperr.ErrPermissionDenied)
	}
	assert.False(t, e.hasRecording(t))
	_, err := e.object(t, 0)
	require.ErrorIs(t, err, fs.ErrNotExist)
}

func TestIngestOtherOrg(t *testing.T) {
	e := newRecEnv(t)
	f2 := dbtest.Seed(t, e.pool)
	gz, hashes := makeChunks(t, 1)
	for _, inst := range []uuid.UUID{f2.InstanceID, e.f.InstanceID} {
		err := e.rec.IngestChunk(t.Context(), f2.OrgID, inst, e.sid, 0, hashes[0], gz[0])
		require.ErrorIs(t, err, apperr.ErrPermissionDenied)
	}
	assert.False(t, e.hasRecording(t))
}

func TestIngestTooLarge(t *testing.T) {
	e := newRecEnv(t)
	big := make([]byte, session.MaxChunkGz+1)
	require.ErrorIs(t, e.ingest(t, 0, hashchain.Next(hashchain.Genesis(), big), big), apperr.ErrInvalidArgument)
	require.ErrorIs(t, e.ingest(t, 0, hashchain.Next(hashchain.Genesis(), nil), nil), apperr.ErrInvalidArgument)
	assert.False(t, e.hasRecording(t))
}
