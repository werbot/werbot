package app

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/sdk"
)

func TestSubscriptionsChannelName(t *testing.T) {
	fn := func(context.Context, string) {}
	for _, c := range []string{"a", "werbot_ee_stub", "a" + strings.Repeat("b", 62)} {
		var s subscriptions
		assert.NotPanics(t, func() { s.add("m", c, fn) }, c)
	}
	for _, c := range []string{"", "A", "1a", "a-b", "a.b", "werbot org", "a" + strings.Repeat("b", 63)} {
		var s subscriptions
		assert.PanicsWithValue(t, `sdk: invalid channel "`+c+`"`, func() { s.add("m", c, fn) }, c)
	}
	var s subscriptions
	assert.PanicsWithValue(t, `sdk: nil handler for channel "x"`, func() { s.add("m", "x", nil) })
}

type ctxKey struct{}

func TestSubscriptionsHandlers(t *testing.T) {
	var calls []string
	ctx := context.WithValue(context.Background(), ctxKey{}, "v")
	rec := func(name string) func(context.Context, string) {
		return func(c context.Context, p string) {
			assert.Equal(t, "v", c.Value(ctxKey{}))
			calls = append(calls, name+":"+p)
		}
	}
	core := map[string]func(string){"c": func(p string) { calls = append(calls, "core:"+p) }}
	var s subscriptions
	s.add("a", "c", rec("a"))
	s.add("boom", "c", func(context.Context, string) { panic("kaboom") })
	s.add("b", "c", rec("b"))
	s.add("a", "d", rec("a-d"))

	var logs bytes.Buffer
	h := s.handlers(ctx, core, slog.New(slog.NewTextHandler(&logs, nil)))
	h["c"]("p")
	assert.Equal(t, []string{"core:p", "a:p", "b:p"}, calls)
	assert.Contains(t, logs.String(), "module subscription panic")
	assert.Contains(t, logs.String(), "module=boom")
	assert.Contains(t, logs.String(), "channel=c")

	calls = nil
	h["d"]("q")
	assert.Equal(t, []string{"a-d:q"}, calls)
	assert.Contains(t, h, "c")

	assert.PanicsWithValue(t, `sdk: Subscribe called outside Init`, func() { s.add("a", "e", rec("e")) })
}

func TestServeModuleSubscribe(t *testing.T) {
	pool, _ := dbtest.New(t)
	freeCh, paidCh := make(chan string, 4), make(chan string, 4)
	var freeHost sdk.Host
	initCalled := false
	modules := []sdk.Module{
		{Name: "free", Init: func(_ context.Context, h sdk.Host) error {
			freeHost = h
			h.Subscribe("werbot_test_free", func(_ context.Context, p string) { freeCh <- p })
			return nil
		}},
		{Name: "paid", Feature: "ee", Init: func(_ context.Context, h sdk.Host) error {
			initCalled = true
			h.Subscribe("werbot_test_paid", func(_ context.Context, p string) { paidCh <- p })
			return nil
		}},
	}
	cfg := config.Config{HTTPAddr: freeAddr(t), WebEmbedded: true}
	startServe(t, cfg, pool, modules, fstest.MapFS{}, slog.New(slog.DiscardHandler))

	var got string
	require.Eventually(t, func() bool {
		for _, n := range [][2]string{{"werbot_test_paid", "x"}, {"werbot_test_free", "hello"}} {
			_, err := pool.Exec(t.Context(), "SELECT pg_notify($1, $2)", n[0], n[1])
			if err != nil {
				return false
			}
		}
		select {
		case got = <-freeCh:
			return true
		default:
			return false
		}
	}, 5*time.Second, 100*time.Millisecond)
	assert.Equal(t, "hello", got)
	assert.Empty(t, paidCh)
	assert.False(t, initCalled)
	assert.PanicsWithValue(t, `sdk: Subscribe called outside Init`, func() {
		freeHost.Subscribe("late", func(context.Context, string) {})
	})
	code, _ := get(t, "http://"+cfg.HTTPAddr+"/readyz")
	assert.Equal(t, http.StatusOK, code)
}
