package policy

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/jobs"
)

var jobsNow = time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)

// jobsEnv is a service over a fresh database and the feed config.
type jobsEnv struct {
	svc  *Service
	pool *pgxpool.Pool
	cfg  JobsConfig
}

func newJobsEnv(t *testing.T) jobsEnv {
	t.Helper()
	pool, _ := dbtest.New(t)
	cfg := DefaultJobsConfig()
	cfg.Now = func() time.Time { return jobsNow }
	return jobsEnv{svc: New(Deps{Pool: pool, Logger: slog.New(slog.DiscardHandler)}), pool: pool, cfg: cfg}
}

// serve returns e with both feed URLs pointing at a server answering h.
func (e jobsEnv) serve(t *testing.T, h http.HandlerFunc) jobsEnv {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	e.cfg.BlocklistURL = srv.URL + "/blocklist"
	e.cfg.GeoIPURL = srv.URL + "/dbip-country-lite-%s.mmdb.gz"
	return e
}

// listen subscribes a separate connection to the GeoIP change channel.
func listen(t *testing.T, pool *pgxpool.Pool) *pgx.Conn {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.ConnectConfig(ctx, pool.Config().ConnConfig)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(ctx) })
	_, err = conn.Exec(ctx, "LISTEN werbot_geoip_changed")
	require.NoError(t, err)
	return conn
}

func TestDailySchedule(t *testing.T) {
	assert.Equal(t, time.Date(2026, 11, 7, 4, 0, 0, 0, time.UTC), dailyAt4{}.Next(jobsNow))
	assert.Equal(t, time.Date(2026, 11, 6, 4, 0, 0, 0, time.UTC), dailyAt4{}.Next(time.Date(2026, 11, 6, 1, 0, 0, 0, time.UTC)))
	assert.Equal(t, time.Date(2027, 1, 1, 4, 0, 0, 0, time.UTC), dailyAt4{}.Next(time.Date(2026, 12, 31, 4, 0, 0, 0, time.UTC)))
}

func TestWorkerTimeoutExceedsHTTPTimeout(t *testing.T) {
	cfg := DefaultJobsConfig()
	assert.Greater(t, NewBlocklistRefreshWorker(nil, cfg).Timeout(nil), cfg.HTTP.Timeout)
	assert.Greater(t, NewGeoIPRefreshWorker(nil, cfg).Timeout(nil), cfg.HTTP.Timeout)
}

func TestRegisterJobs(t *testing.T) {
	periodic := RegisterJobs(river.NewWorkers(), New(Deps{}), DefaultJobsConfig())
	assert.Len(t, periodic, 2)
	assert.Equal(t, jobs.QueueFeeds, BlocklistRefreshArgs{}.InsertOpts().Queue)
	assert.Equal(t, jobs.QueueFeeds, GeoIPRefreshArgs{}.InsertOpts().Queue)
}
