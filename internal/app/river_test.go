package app

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"testing/fstest"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/notify"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/jobs"
	"github.com/werbot/werbot/internal/platform/secretbox"
)

// blockingHandler blocks the log mailer's "email" record until release closes.
type blockingHandler struct {
	slog.Handler
	started chan struct{}
	release chan struct{}
	ctxErr  *error // job ctx error observed once released
}

func (h blockingHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Message == "email (log mailer)" {
		close(h.started)
		<-h.release
		*h.ctxErr = ctx.Err()
	}
	return h.Handler.Handle(ctx, r)
}

// A job running when serve's ctx is cancelled must finish: River is started
// without ctx and stopped (10s) only after HTTP Shutdown.
func TestServeRunningJobSurvivesCancel(t *testing.T) {
	pool, _ := dbtest.New(t)
	cfg := config.Config{HTTPAddr: freeAddr(t), WebEmbedded: true}
	withSecretKey(&cfg)
	var ctxErr error
	h := blockingHandler{Handler: slog.NewTextHandler(io.Discard, nil), started: make(chan struct{}), release: make(chan struct{}), ctxErr: &ctxErr}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serve(ctx, cfg, slog.New(h), pool, nil, fstest.MapFS{}) }()
	require.Eventually(t, func() bool {
		res, err := http.Get("http://" + cfg.HTTPAddr + "/healthz")
		if err != nil {
			return false
		}
		_ = res.Body.Close()
		return res.StatusCode == http.StatusOK
	}, 5*time.Second, 20*time.Millisecond)

	box, err := secretbox.New(cfg.SecretKey, nil)
	require.NoError(t, err)
	workers := river.NewWorkers()
	river.AddWorker(workers, notify.NewWorker(nil, box))                             // only registers the kind for InsertTx
	client, err := jobs.NewClient(pool, workers, nil, slog.New(slog.DiscardHandler)) // insert only, never started
	require.NoError(t, err)
	require.NoError(t, dbx.InOrg(ctx, pool, uuid.New(), uuid.Nil, func(tx pgx.Tx) error {
		return notify.NewQueue(client, box).EnqueueTx(ctx, tx, "a@example.com", "password_reset", map[string]string{"url": "https://w.example/r", "name": "Ann"})
	}))
	select {
	case <-h.started:
	case <-time.After(10 * time.Second):
		t.Fatal("email job did not start")
	}

	cancel()
	select {
	case err := <-done:
		t.Fatalf("serve returned while a job was running: %v", err)
	case <-time.After(500 * time.Millisecond):
	}
	close(h.release)
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not return after the job finished")
	}

	assert.NoError(t, ctxErr, "running job context must not be cancelled with serve's ctx")
	var state string
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT state::text FROM river_job WHERE kind = 'send_email'").Scan(&state))
	assert.Equal(t, "completed", state)
}
