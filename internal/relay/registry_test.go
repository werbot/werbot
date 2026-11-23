package relay

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/jobs"
	"github.com/werbot/werbot/internal/platform/secretbox"
	"github.com/werbot/werbot/internal/platform/tlsca"
)

// memCA keeps one test CA in memory.
type memCA struct {
	mu sync.Mutex
	s  *tlsca.StoredCA
}

func (m *memCA) Load(context.Context) (tlsca.StoredCA, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.s == nil {
		return tlsca.StoredCA{}, false, nil
	}
	return *m.s, true, nil
}

func (m *memCA) Insert(_ context.Context, s tlsca.StoredCA) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.s == nil {
		m.s = &s
	}
	return nil
}

func newCA(t *testing.T) *tlsca.CA {
	t.Helper()
	box, err := secretbox.New(make([]byte, 32), nil)
	require.NoError(t, err)
	ca, err := tlsca.LoadOrCreate(context.Background(), &memCA{}, box)
	require.NoError(t, err)
	return ca
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	return addr
}

// newRelay builds a relay on a free loopback port with a 50ms heartbeat.
func newRelay(t *testing.T, pool *pgxpool.Pool, ca *tlsca.CA) *Relay {
	t.Helper()
	addr := freeAddr(t)
	r := New(pool, ca, Config{InstanceID: uuid.New(), InternalAddr: addr, InternalURL: "https://" + addr, Version: "v-test"},
		slog.New(slog.DiscardHandler))
	r.heartbeat = 50 * time.Millisecond
	return r
}

// start runs r until the test ends and waits for its first registration.
func start(t *testing.T, r *Relay) (stop func() error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	require.Eventually(t, r.registered.Load, 5*time.Second, 10*time.Millisecond)
	var once sync.Once
	var err error
	stop = func() error {
		once.Do(func() { cancel(); err = <-done })
		return err
	}
	t.Cleanup(func() { _ = stop() })
	return stop
}

func lastSeen(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) (time.Time, bool) {
	t.Helper()
	var ts time.Time
	err := pool.QueryRow(context.Background(), "SELECT last_seen_at FROM server_instance WHERE id = $1", id).Scan(&ts)
	if err != nil {
		return time.Time{}, false
	}
	return ts, true
}

func TestRunRegistersAndDeregisters(t *testing.T) {
	pool, _ := dbtest.New(t)
	r := newRelay(t, pool, newCA(t))
	stop := start(t, r)
	ctx := context.Background()

	host, err := os.Hostname()
	require.NoError(t, err)
	var gotHost, gotURL, gotVersion string
	require.NoError(t, pool.QueryRow(ctx, "SELECT hostname, internal_url, version FROM server_instance WHERE id = $1",
		r.cfg.InstanceID).Scan(&gotHost, &gotURL, &gotVersion))
	assert.Equal(t, host, gotHost)
	assert.Equal(t, r.cfg.InternalURL, gotURL)
	assert.Equal(t, "v-test", gotVersion)

	first, _ := lastSeen(t, pool, r.cfg.InstanceID)
	require.Eventually(t, func() bool {
		ts, ok := lastSeen(t, pool, r.cfg.InstanceID)
		return ok && ts.After(first)
	}, 2*time.Second, 10*time.Millisecond, "heartbeat must move last_seen_at")

	_, err = pool.Exec(ctx, "DELETE FROM server_instance WHERE id = $1", r.cfg.InstanceID)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		_, ok := lastSeen(t, pool, r.cfg.InstanceID)
		return ok
	}, 200*time.Millisecond, 5*time.Millisecond, "heartbeat after GC registers again")

	require.NoError(t, stop())
	_, ok := lastSeen(t, pool, r.cfg.InstanceID)
	assert.False(t, ok, "row removed on exit")
}

func TestRunFailsOnBusyAddress(t *testing.T) {
	pool, _ := dbtest.New(t)
	r := newRelay(t, pool, newCA(t))
	ln, err := net.Listen("tcp", r.cfg.InternalAddr)
	require.NoError(t, err)
	defer ln.Close()
	require.Error(t, r.Run(context.Background()))
	_, ok := lastSeen(t, pool, r.cfg.InstanceID)
	assert.False(t, ok, "no row left behind")
}

func TestOnlineURLOfflineAfterWindow(t *testing.T) {
	pool, _ := dbtest.New(t)
	r := newRelay(t, pool, newCA(t))
	ctx := context.Background()
	peer := uuid.New()
	_, err := pool.Exec(ctx, "INSERT INTO server_instance (id, hostname, internal_url) VALUES ($1, 'b', 'https://b:8444')", peer)
	require.NoError(t, err)

	got, err := r.onlineURL(ctx, peer)
	require.NoError(t, err)
	assert.Equal(t, "https://b:8444", got)

	_, err = pool.Exec(ctx, "UPDATE server_instance SET last_seen_at = now() - interval '44 seconds' WHERE id = $1", peer)
	require.NoError(t, err)
	_, err = r.onlineURL(ctx, peer)
	require.NoError(t, err, "44s without heartbeat is still online")

	_, err = pool.Exec(ctx, "UPDATE server_instance SET last_seen_at = now() - interval '46 seconds' WHERE id = $1", peer)
	require.NoError(t, err)
	_, err = r.onlineURL(ctx, peer)
	assert.True(t, errors.Is(err, apperr.ErrUnavailable), "46s without heartbeat is offline: %v", err)

	_, err = r.onlineURL(ctx, uuid.New())
	assert.True(t, errors.Is(err, apperr.ErrUnavailable), "unknown replica: %v", err)
}

func TestGCDeletesStaleInstances(t *testing.T) {
	pool, _ := dbtest.New(t)
	r := newRelay(t, pool, newCA(t))
	ctx := context.Background()
	stale, fresh := uuid.New(), uuid.New()
	for id, age := range map[uuid.UUID]string{stale: "11 minutes", fresh: "9 minutes"} {
		_, err := pool.Exec(ctx, "INSERT INTO server_instance (id, hostname, internal_url, last_seen_at) VALUES ($1, 'h', 'https://h:8444', now() - $2::interval)", id, age)
		require.NoError(t, err)
	}
	n, err := r.GC(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
	_, ok := lastSeen(t, pool, stale)
	assert.False(t, ok)
	_, ok = lastSeen(t, pool, fresh)
	assert.True(t, ok)

	periodic := RegisterJobs(river.NewWorkers(), r)
	assert.Len(t, periodic, 1)
	assert.Equal(t, "server_instance.gc", GCArgs{}.Kind())
	assert.Equal(t, jobs.QueueMaintenance, GCArgs{}.InsertOpts().Queue)
}
