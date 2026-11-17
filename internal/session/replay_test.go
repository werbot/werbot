package session_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/hashchain"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/session"
)

const verifiedHeader = "X-Werbot-Recording-Verified"

// replay sends GET path to the replay route, as p when p is not nil.
func (e recEnv) replay(t *testing.T, p *principal.Principal, path string) *httptest.ResponseRecorder {
	t.Helper()
	return e.serve(t, request(t.Context(), p, path), t.Output())
}

// serve passes r to the replay route, which logs to logs.
func (e recEnv) serve(t *testing.T, r *http.Request, logs io.Writer) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /api/recordings/{file}", session.NewReplayHandler(session.New(e.pool), e.rec, slog.New(slog.NewTextHandler(logs, nil))))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

func request(ctx context.Context, p *principal.Principal, path string) *http.Request {
	if p != nil {
		ctx = principal.With(ctx, *p)
	}
	return httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil)
}

func gzipOf(t *testing.T, raw []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := gzip.NewWriter(&b)
	_, err := zw.Write(raw)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return b.Bytes()
}

func (e recEnv) as(userID uuid.UUID) *principal.Principal {
	p := e.f.Principal(userID)
	return &p
}

func castPath(id uuid.UUID) string { return "/api/recordings/" + id.String() + ".cast" }

func TestReplayOwnerVerified(t *testing.T) {
	e, _ := finishedEnv(t)
	w := e.replay(t, e.as(e.f.OwnerID), castPath(e.sid))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/x-asciicast", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, `attachment; filename="`+e.sid.String()+`.cast"`, w.Header().Get("Content-Disposition"))
	assert.Equal(t, "true", w.Header().Get(verifiedHeader))
	assert.Equal(t, "chunk-0chunk-1chunk-2", w.Body.String())
}

func TestReplayVisibility(t *testing.T) {
	e, _ := finishedEnv(t)
	other := dbtest.Seed(t, e.pool)
	foreignAdmin := other.Principal(other.AdminID)
	for name, tc := range map[string]struct {
		p    *principal.Principal
		code int
	}{
		"member own":         {e.as(e.f.MemberID), http.StatusOK},
		"other member":       {e.as(e.f.OtherMemberID), http.StatusNotFound},
		"admin":              {e.as(e.f.AdminID), http.StatusOK},
		"admin of other org": {&foreignAdmin, http.StatusNotFound},
		"no principal":       {nil, http.StatusUnauthorized},
	} {
		t.Run(name, func(t *testing.T) {
			w := e.replay(t, tc.p, castPath(e.sid))
			assert.Equal(t, tc.code, w.Code)
			if tc.code != http.StatusOK {
				assert.Empty(t, w.Header().Get(verifiedHeader))
			}
		})
	}
}

func TestReplayTampered(t *testing.T) {
	for name, tc := range map[string]struct {
		tamper func(t *testing.T, e recEnv, dir string)
		body   string // served as stored: exactly the bytes that failed verification
	}{
		"chunk 1 overwritten": {func(t *testing.T, e recEnv, dir string) {
			require.NoError(t, os.WriteFile(e.path(dir, 1), gzipOf(t, []byte("forged")), 0o600))
		}, "chunk-0forgedchunk-2"},
		"chunk 2 deleted": {func(t *testing.T, e recEnv, dir string) {
			require.NoError(t, os.Remove(e.path(dir, 2)))
		}, "chunk-0chunk-1"},
		"chunk 0 deleted": {func(t *testing.T, e recEnv, dir string) {
			require.NoError(t, os.Remove(e.path(dir, 0)))
		}, ""},
		"chunks 0 and 1 swapped": {func(t *testing.T, e recEnv, dir string) {
			a, b := e.path(dir, 0), e.path(dir, 1)
			require.NoError(t, os.Rename(a, a+".tmp"))
			require.NoError(t, os.Rename(b, a))
			require.NoError(t, os.Rename(a+".tmp", b))
		}, "chunk-1chunk-0chunk-2"},
	} {
		t.Run(name, func(t *testing.T) {
			e, dir := finishedEnv(t)
			tc.tamper(t, e, dir)
			w := e.replay(t, e.as(e.f.OwnerID), castPath(e.sid))
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "false", w.Header().Get(verifiedHeader))
			assert.Equal(t, tc.body, w.Body.String())
		})
	}
}

// A forged chunk that inflates past the per-chunk cap ends the response there.
func TestReplayBombChunk(t *testing.T) {
	e := newRecEnv(t)
	gz, _ := makeChunks(t, 3)
	gz[1] = gzipOf(t, make([]byte, 4<<20))
	require.LessOrEqual(t, len(gz[1]), session.MaxChunkGz)
	prev, hashes := hashchain.Genesis(), [][]byte{}
	for _, g := range gz {
		prev = hashchain.Next(prev, g)
		hashes = append(hashes, prev)
	}
	e.ingestAll(t, gz, hashes)
	require.NoError(t, e.finish(t, 3, sumLen(gz), hashes[2]))

	var logs bytes.Buffer
	w := e.serve(t, request(t.Context(), e.as(e.f.OwnerID), castPath(e.sid)), &logs)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "true", w.Header().Get(verifiedHeader))
	body := w.Body.String()
	assert.True(t, strings.HasPrefix(body, "chunk-0\x00"))
	assert.Less(t, len(body), 2<<20)
	assert.NotContains(t, body, "chunk-2")
	assert.Contains(t, logs.String(), "recording cut short")
}

// A client that went away is not a server error worth a log line.
func TestReplayClientGoneNotLogged(t *testing.T) {
	e, _ := finishedEnv(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var logs bytes.Buffer
	e.serve(t, request(ctx, e.as(e.f.OwnerID), castPath(e.sid)), &logs)
	assert.Empty(t, logs.String())
}

func TestReplayUnfinished(t *testing.T) {
	e := newRecEnv(t)
	gz, hashes := makeChunks(t, 2)
	e.ingestAll(t, gz, hashes)
	w := e.replay(t, e.as(e.f.MemberID), castPath(e.sid))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "false", w.Header().Get(verifiedHeader))
	assert.Equal(t, "chunk-0chunk-1", w.Body.String())
}

func TestReplayNoRecording(t *testing.T) {
	e := newRecEnv(t)
	w := e.replay(t, e.as(e.f.MemberID), castPath(e.sid))
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Empty(t, w.Header().Get(verifiedHeader))
}

func TestReplayBadPath(t *testing.T) {
	e, _ := finishedEnv(t)
	for _, path := range []string{
		"/api/recordings/abc.cast",
		"/api/recordings/" + e.sid.String() + ".txt",
		"/api/recordings/" + e.sid.String(),
		castPath(uuid.New()),
	} {
		t.Run(path, func(t *testing.T) {
			assert.Equal(t, http.StatusNotFound, e.replay(t, e.as(e.f.OwnerID), path).Code)
		})
	}
}

func TestReplayStoreUnavailable(t *testing.T) {
	e, _ := finishedEnv(t)
	e.store.down.Store(true)
	w := e.replay(t, e.as(e.f.OwnerID), castPath(e.sid))
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	_, ok := w.Header()[verifiedHeader]
	assert.False(t, ok)
}
