package app

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"testing/fstest"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/version"
	"github.com/werbot/werbot/internal/webui"
)

func TestWebProxiesGetVersion(t *testing.T) {
	pool, _ := dbtest.New(t)
	scfg := config.Config{HTTPAddr: freeAddr(t), WebEmbedded: false}
	stopServer := startServe(t, scfg, pool, nil, fstest.MapFS{}, slog.New(slog.DiscardHandler))

	ui := fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}
	wcfg := webui.Config{Addr: freeAddr(t), APIURL: "http://" + scfg.HTTPAddr}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- webui.Run(ctx, wcfg, ui, slog.New(slog.DiscardHandler)) }()
	web := "http://" + wcfg.Addr
	require.Eventually(t, func() bool {
		res, err := http.Get(web + "/healthz")
		if err != nil {
			return false
		}
		_ = res.Body.Close()
		return res.StatusCode == http.StatusOK
	}, 5*time.Second, 20*time.Millisecond)

	client := werbotv1connect.NewSystemServiceClient(http.DefaultClient, web)
	res, err := client.GetVersion(context.Background(), connect.NewRequest(&werbotv1.GetVersionRequest{}))
	require.NoError(t, err)
	assert.Equal(t, version.Version, res.Msg.GetVersion())

	code, body := getHTML(t, web+"/")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "<html>app</html>", body)
	code, _ = getHTML(t, "http://"+scfg.HTTPAddr+"/")
	assert.Equal(t, http.StatusNotFound, code)

	stopServer()
	_, err = client.GetVersion(context.Background(), connect.NewRequest(&werbotv1.GetVersionRequest{}))
	assert.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))
	code, body = get(t, web+"/readyz")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "ok", body)

	cancel()
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("webui.Run did not return within 10s")
	}
}

func TestRunWebMissingAPIURL(t *testing.T) {
	assert.NotPanics(t, func() {
		err := Run(context.Background(), []string{"web"}, env(nil), io.Discard)
		assert.ErrorContains(t, err, "WERBOT_WEB_API_URL")
	})
}
