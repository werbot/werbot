package session_test

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/session"
)

const verifiedHeader = "X-Werbot-Recording-Verified"

// replay sends GET path to the replay route, as p when p is not nil.
func (e recEnv) replay(t *testing.T, p *principal.Principal, path string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /api/recordings/{file}", session.NewReplayHandler(session.New(e.pool), e.rec))
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	if p != nil {
		r = r.WithContext(principal.With(r.Context(), *p))
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
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
	for name, tamper := range map[string]func(t *testing.T, e recEnv, dir string){
		"chunk 1 overwritten": func(t *testing.T, e recEnv, dir string) {
			var b bytes.Buffer
			zw := gzip.NewWriter(&b)
			_, err := zw.Write([]byte("forged"))
			require.NoError(t, err)
			require.NoError(t, zw.Close())
			require.NoError(t, os.WriteFile(e.path(dir, 1), b.Bytes(), 0o600))
		},
		"chunk 2 deleted": func(t *testing.T, e recEnv, dir string) {
			require.NoError(t, os.Remove(e.path(dir, 2)))
		},
		"chunk 0 deleted": func(t *testing.T, e recEnv, dir string) {
			require.NoError(t, os.Remove(e.path(dir, 0)))
		},
		"chunks 0 and 1 swapped": func(t *testing.T, e recEnv, dir string) {
			a, b := e.path(dir, 0), e.path(dir, 1)
			require.NoError(t, os.Rename(a, a+".tmp"))
			require.NoError(t, os.Rename(b, a))
			require.NoError(t, os.Rename(a+".tmp", b))
		},
	} {
		t.Run(name, func(t *testing.T) {
			e, dir := finishedEnv(t)
			tamper(t, e, dir)
			w := e.replay(t, e.as(e.f.OwnerID), castPath(e.sid))
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "false", w.Header().Get(verifiedHeader))
		})
	}
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
