package session_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	_ "github.com/werbot/werbot/internal/connector/postgres" // registers the querylog type
	_ "github.com/werbot/werbot/internal/connector/ssh"      // registers an asciicast type
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/hashchain"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/session"
)

// asStream turns the session of e into a stream session on a resource of type typ.
func (e recEnv) asStream(t *testing.T, typ string) {
	t.Helper()
	_, err := e.pool.Exec(t.Context(), "UPDATE resource SET type = $2 WHERE id = $1", e.f.ResourceID, typ)
	require.NoError(t, err)
	_, err = e.pool.Exec(t.Context(), "UPDATE session SET kind = 'stream' WHERE id = $1", e.sid)
	require.NoError(t, err)
}

// recording returns the format, store and chunks of the recording row of e, ok false without one.
func (e recEnv) recording(t *testing.T) (format, store string, chunks int, ok bool) {
	t.Helper()
	ok = e.hasRecording(t)
	if ok {
		require.NoError(t, e.pool.QueryRow(t.Context(),
			"SELECT format, store, chunks FROM recording WHERE session_id = $1", e.sid).Scan(&format, &store, &chunks))
	}
	return format, store, chunks, ok
}

func (e recEnv) begin(t *testing.T) error {
	t.Helper()
	return e.rec.Begin(t.Context(), e.f.OrgID, e.sid)
}

// Begin opens a querylog recording for a recorded session of a querylog type,
// once; other types and kinds stay lazy (asciicast, opened by the first chunk).
func TestBeginQuerylog(t *testing.T) {
	e := newRecEnv(t)
	e.asStream(t, "postgres")
	require.NoError(t, e.begin(t))
	require.NoError(t, e.begin(t), "a repeat is a no-op")
	format, store, chunks, ok := e.recording(t)
	require.True(t, ok)
	assert.Equal(t, [3]any{"querylog", "fs", 0}, [3]any{format, store, chunks})

	assert.ErrorIs(t, e.rec.Begin(t.Context(), uuid.New(), e.sid), apperr.ErrNotFound, "a session of another organization")

	ssh := newRecEnv(t) // a terminal session of an ssh resource
	require.NoError(t, ssh.begin(t))
	_, _, _, ok = ssh.recording(t)
	assert.False(t, ok, "asciicast recordings start with the first chunk")

	notRecorded := newRecEnv(t) // a postgres resource, a kind its type does not record
	_, err := notRecorded.pool.Exec(t.Context(), "UPDATE resource SET type = 'postgres' WHERE id = $1", notRecorded.f.ResourceID)
	require.NoError(t, err)
	require.NoError(t, notRecorded.begin(t))
	_, _, _, ok = notRecorded.recording(t)
	assert.False(t, ok)
}

// The first chunk of a begun querylog recording keeps its format.
func TestIngestKeepsQuerylog(t *testing.T) {
	e := newRecEnv(t)
	e.asStream(t, "postgres")
	require.NoError(t, e.begin(t))
	gz, hashes := makeChunks(t, 1)
	require.NoError(t, e.ingest(t, 0, hashes[0], gz[0]))
	format, _, chunks, _ := e.recording(t)
	assert.Equal(t, [2]any{"querylog", 1}, [2]any{format, chunks})
}

// querylogEnv is a begun querylog recording of two NDJSON chunks, not finished.
func querylogEnv(t *testing.T) (e recEnv, lines [][]byte, last []byte) {
	t.Helper()
	e = newRecEnv(t)
	e.asStream(t, "postgres")
	require.NoError(t, e.begin(t))
	lines = [][]byte{[]byte(`{"t":0.1,"kind":"query","sql":"SELECT 1"}` + "\n"), []byte(`{"t":0.2,"kind":"query","sql":"SELECT 2"}` + "\n")}
	last = hashchain.Genesis()
	for i, l := range lines {
		gz := gzipOf(t, l)
		last = hashchain.Next(last, gz)
		require.NoError(t, e.ingest(t, i, last, gz))
	}
	return e, lines, last
}

func querylogPath(id uuid.UUID) string { return "/api/recordings/" + id.String() + ".querylog" }

// A querylog recording replays as NDJSON at <id>.querylog with the same chain
// check and the same access as a .cast; an extension that is not the
// session's recording format is not found.
func TestReplayQuerylog(t *testing.T) {
	e, lines, last := querylogEnv(t)
	w := e.replay(t, e.as(e.f.OwnerID), querylogPath(e.sid))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/x-ndjson", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	assert.Equal(t, `attachment; filename="`+e.sid.String()+`.querylog"`, w.Header().Get("Content-Disposition"))
	assert.Equal(t, "false", w.Header().Get(verifiedHeader), "not finished")
	assert.Equal(t, string(lines[0])+string(lines[1]), w.Body.String())

	var size int64
	require.NoError(t, e.pool.QueryRow(t.Context(), "SELECT bytes FROM recording WHERE session_id = $1", e.sid).Scan(&size))
	require.NoError(t, e.finish(t, 2, size, last))
	w = e.replay(t, e.as(e.f.OwnerID), querylogPath(e.sid))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "true", w.Header().Get(verifiedHeader))

	assert.Equal(t, http.StatusNotFound, e.replay(t, e.as(e.f.OwnerID), castPath(e.sid)).Code, ".cast of a querylog recording")
	cast, _ := finishedEnv(t)
	assert.Equal(t, http.StatusNotFound, cast.replay(t, cast.as(cast.f.OwnerID), querylogPath(cast.sid)).Code, ".querylog of an asciicast recording")

	other := dbtest.Seed(t, e.pool)
	foreignAdmin := other.Principal(other.AdminID)
	for name, tc := range map[string]struct {
		p    *principal.Principal
		code int
	}{
		"member own":         {e.as(e.f.MemberID), http.StatusOK},
		"other member":       {e.as(e.f.OtherMemberID), http.StatusNotFound},
		"admin of other org": {&foreignAdmin, http.StatusNotFound},
		"no principal":       {nil, http.StatusUnauthorized},
	} {
		t.Run(name, func(t *testing.T) {
			w := e.replay(t, tc.p, querylogPath(e.sid))
			assert.Equal(t, tc.code, w.Code)
			if tc.code != http.StatusOK {
				assert.Empty(t, w.Header().Get(verifiedHeader))
			}
		})
	}
}

// The API reports the query count and the recording format; a session
// without a recording has an empty format.
func TestSessionQueriesFormatAPI(t *testing.T) {
	e, _, _ := querylogEnv(t)
	_, err := e.pool.Exec(t.Context(), "UPDATE session SET queries = 7 WHERE id = $1", e.sid)
	require.NoError(t, err)
	bare := e.f.StartSession(t, e.pool, e.f.MemberID)
	c := rpcClient(t, e, map[string]principal.Principal{"admin": e.f.Principal(e.f.AdminID)})

	get := func(id uuid.UUID) *werbotv1.Session {
		res, err := c.GetSession(context.Background(), as("admin", &werbotv1.SessionServiceGetSessionRequest{Id: id.String()}))
		require.NoError(t, err)
		return res.Msg.GetSession()
	}
	s := get(e.sid)
	assert.Equal(t, [2]any{int64(7), "querylog"}, [2]any{s.GetQueries(), s.GetRecordFormat()})
	s = get(bare)
	assert.Equal(t, [2]any{int64(0), ""}, [2]any{s.GetQueries(), s.GetRecordFormat()})

	cast, _ := finishedEnv(t)
	sum, err := session.New(cast.pool).GetSession(t.Context(), cast.f.Principal(cast.f.OwnerID), cast.sid)
	require.NoError(t, err)
	assert.Equal(t, "asciicast", sum.RecordFormat)
}
