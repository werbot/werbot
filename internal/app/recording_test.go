package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway/recorder"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/platform/secretbox"
	"github.com/werbot/werbot/internal/session"
	"github.com/werbot/werbot/internal/session/recstore"
	"github.com/werbot/werbot/internal/webui"
)

// flakyStore fails Put and Get while down is set (copy of the session package test helper).
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

// ingestUploader is the server side of the gateway tunnel for one instance:
// Unavailable while down, else the calls the tunnel handler makes.
type ingestUploader struct {
	rec               *session.Recordings
	sessions          *session.Service
	orgID, instanceID uuid.UUID
	down              atomic.Bool
}

func (u *ingestUploader) unavailable() error {
	if u.down.Load() {
		return connect.NewError(connect.CodeUnavailable, errors.New("server down"))
	}
	return nil
}

func (u *ingestUploader) UploadChunk(ctx context.Context, sid uuid.UUID, seq int, hash, gz []byte) error {
	if err := u.unavailable(); err != nil {
		return err
	}
	return apperr.ToConnect(u.rec.IngestChunk(ctx, u.orgID, u.instanceID, sid, seq, hash, gz))
}

func (u *ingestUploader) EndSession(ctx context.Context, sid uuid.UUID, reason string) error {
	if err := u.unavailable(); err != nil {
		return err
	}
	return apperr.ToConnect(u.sessions.End(ctx, u.orgID, u.instanceID, sid, reason))
}

func (u *ingestUploader) FinishRecording(ctx context.Context, m recorder.Manifest) error {
	if err := u.unavailable(); err != nil {
		return err
	}
	return apperr.ToConnect(u.rec.FinishRecording(ctx, u.orgID, u.instanceID, m.SessionID, m.Chunks, m.Bytes, m.FinalHash))
}

// countingSink counts the chunks a recorder hands to the spool.
type countingSink struct {
	*recorder.Spool
	n atomic.Int64
}

func (c *countingSink) Append(sid uuid.UUID, seq int, hash, gz []byte) error {
	c.n.Add(1)
	return c.Spool.Append(sid, seq, hash, gz)
}

// outageRig is a gateway spool delivering to the server services of one
// seeded organization; sid is an open session of f.MemberID.
type outageRig struct {
	pool     *pgxpool.Pool
	f        dbtest.Fixture
	sid      uuid.UUID
	store    *flakyStore
	up       *ingestUploader
	rec      *session.Recordings
	sessions *session.Service
	sink     *countingSink
}

func newOutageRig(t *testing.T) *outageRig {
	t.Helper()
	pool, _ := dbtest.New(t)
	f := dbtest.Seed(t, pool)
	box, err := secretbox.New(bytes.Repeat([]byte{1}, 32), nil)
	require.NoError(t, err)
	signer, err := session.LoadSigner(t.Context(), pool, box)
	require.NoError(t, err)
	g := &outageRig{pool: pool, f: f, sid: f.StartSession(t, pool, f.MemberID),
		store: &flakyStore{RecordingStore: recstore.NewFS(t.TempDir())}, sessions: session.New(pool)}
	g.rec = session.NewRecordings(pool, g.store, "fs", signer)
	g.up = &ingestUploader{rec: g.rec, sessions: g.sessions, orgID: f.OrgID, instanceID: f.InstanceID}
	sp, err := recorder.OpenSpool(t.TempDir(), g.up,
		recorder.SpoolOptions{RetryMin: 10 * time.Millisecond, RetryMax: 50 * time.Millisecond}, slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	g.sink = &countingSink{Spool: sp}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = sp.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done; _ = sp.Shutdown() })
	return g
}

const recordedLine = "werbot recording line\r\n"

// record plays a 200 KiB session that ends with "closed".
func (g *outageRig) record(t *testing.T) {
	t.Helper()
	r := recorder.New(g.sid, 80, 24, g.sink, recorder.Options{})
	r.Output([]byte(strings.Repeat(recordedLine, 200<<10/len(recordedLine))))
	require.NoError(t, g.sink.End(g.sid, "closed"))
	require.NoError(t, r.Close(t.Context()))
	require.Greater(t, g.sink.n.Load(), int64(1))
}

func (g *outageRig) verified(t *testing.T) bool {
	v, err := g.rec.Verify(t.Context(), g.f.Principal(g.f.OwnerID), g.sid)
	return err == nil && v.Verified
}

// endReason is the session's end_reason, "" while it is open. It uses assert,
// not require: it also runs inside require.Eventually, off the test goroutine.
func (g *outageRig) endReason(t *testing.T) string {
	var reason *string
	err := g.pool.QueryRow(t.Context(), "SELECT end_reason FROM session WHERE id = $1", g.sid).Scan(&reason)
	if !assert.NoError(t, err) || reason == nil {
		return ""
	}
	return *reason
}

// api serves the mounted routes as the session's member, as after identity.Middleware.
func (g *outageRig) api(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()
	(&services{Sessions: g.sessions, Recordings: g.rec}).mount(mux)
	p := g.f.Principal(g.f.MemberID)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r.WithContext(principal.With(r.Context(), p)))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// httpGet returns the status, headers and body of GET url.
func httpGet(t *testing.T, url string) (int, http.Header, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	require.NoError(t, err)
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	return res.StatusCode, res.Header, string(body)
}

func (g *outageRig) castPath() string { return "/api/recordings/" + g.sid.String() + ".cast" }

func TestRecordingSurvivesServerOutage(t *testing.T) {
	g := newOutageRig(t)
	g.up.down.Store(true)
	g.record(t)
	assert.Positive(t, g.sink.Pending(g.sid))

	// the server's heartbeat check sees the session in the gateway's pending ends
	_, err := g.pool.Exec(t.Context(), "UPDATE session SET started_at = now() - interval '2 minutes' WHERE id = $1", g.sid)
	require.NoError(t, err)
	n, err := g.sessions.EndMissing(t.Context(), g.f.OrgID, g.f.InstanceID, g.sink.PendingEnds(), "gateway_lost")
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Empty(t, g.endReason(t))

	g.up.down.Store(false)
	require.Eventually(t, func() bool { return g.verified(t) }, 5*time.Second, 20*time.Millisecond)
	assert.Equal(t, "closed", g.endReason(t))

	code, h, body := httpGet(t, g.api(t).URL+g.castPath())
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "true", h.Get("X-Werbot-Recording-Verified"))
	first, _, _ := strings.Cut(body, "\n")
	assert.Contains(t, first, `"version":2`)
	assert.Contains(t, body, "werbot recording line")
}

func TestRecordingSurvivesStoreOutage(t *testing.T) {
	g := newOutageRig(t)
	g.store.down.Store(true)
	g.record(t)
	require.Eventually(t, func() bool { return g.endReason(t) == "closed" }, 5*time.Second, 20*time.Millisecond)
	assert.Positive(t, g.sink.Pending(g.sid))
	var exists bool
	require.NoError(t, g.pool.QueryRow(t.Context(), "SELECT EXISTS (SELECT 1 FROM recording WHERE session_id = $1)", g.sid).Scan(&exists))
	assert.False(t, exists)

	g.store.down.Store(false)
	require.Eventually(t, func() bool { return g.verified(t) && g.sink.Total() == 0 }, 5*time.Second, 20*time.Millisecond)
	var chunks int64
	require.NoError(t, g.pool.QueryRow(t.Context(), "SELECT chunks FROM recording WHERE session_id = $1", g.sid).Scan(&chunks))
	assert.Equal(t, g.sink.n.Load(), chunks)
}

func TestReplayThroughWeb(t *testing.T) {
	g := newOutageRig(t)
	g.record(t)
	require.Eventually(t, func() bool { return g.verified(t) }, 5*time.Second, 20*time.Millisecond)

	api := g.api(t)
	var draining atomic.Bool
	h, err := webui.Handler(webui.Config{APIURL: api.URL}, fstest.MapFS{}, &draining, metrics.NewRegistry(), slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	web := httptest.NewServer(h)
	t.Cleanup(web.Close)

	directCode, direct, want := httpGet(t, api.URL+g.castPath())
	proxiedCode, proxied, got := httpGet(t, web.URL+g.castPath())
	require.Equal(t, http.StatusOK, directCode)
	require.Equal(t, http.StatusOK, proxiedCode)
	assert.Equal(t, want, got)
	for k, v := range map[string]string{
		"Content-Type": "application/x-asciicast", "X-Werbot-Recording-Verified": "true", "Cache-Control": "no-store",
	} {
		assert.Equal(t, v, direct.Get(k), k)
		assert.Equal(t, v, proxied.Get(k), k)
	}
}
