package cli

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandsConnect(t *testing.T) {
	env := Env{Getenv: func(string) string { return "" }, Stdout: io.Discard, Stderr: io.Discard}
	cmds := Commands(env)
	require.Len(t, cmds, 1)
	assert.Equal(t, "connect", cmds[0].Name())
	cmds[0].SetContext(context.Background())
	err := cmds[0].RunE(cmds[0], []string{"pg"})
	assert.ErrorContains(t, err, "WERBOT_CONNECT_GATEWAY")
}

func TestBrowserCommand(t *testing.T) {
	for goos, want := range map[string][]string{
		"linux":   {"xdg-open", "u"},
		"darwin":  {"open", "u"},
		"windows": {"rundll32", "url.dll,FileProtocolHandler", "u"},
	} {
		assert.Equal(t, want, browserCommand(goos, "u"), goos)
	}
}
