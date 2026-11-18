package app

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunConnectUsage(t *testing.T) {
	err := Run(context.Background(), []string{"bogus"}, env(nil), io.Discard)
	assert.EqualError(t, err, "usage: werbot <server|gateway|web|connect|migrate|version|healthcheck>")
	err = Run(context.Background(), []string{"connect", "pg"}, env(nil), io.Discard)
	assert.ErrorContains(t, err, "WERBOT_CONNECT_GATEWAY")
	err = Run(context.Background(), []string{"connect"}, env(map[string]string{"WERBOT_CONNECT_GATEWAY": "gw", "WERBOT_CONNECT_LOGIN": "alice"}), io.Discard)
	assert.ErrorContains(t, err, "usage: werbot connect [flags] [<local_port>:]<resource>[:<target_port>] ...")
	assert.NoError(t, Run(context.Background(), []string{"connect", "-h"}, env(nil), io.Discard), "help is not an error")
}
