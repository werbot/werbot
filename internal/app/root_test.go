package app

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootHelpListsCommands(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, Run(context.Background(), []string{"--help"}, env(nil), &out))
	for _, cmd := range []string{"server", "gateway", "web", "connect", "migrate", "version", "healthcheck"} {
		assert.Contains(t, out.String(), cmd)
	}
}

func TestRootNoArgsPrintsHelp(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, Run(context.Background(), nil, env(nil), &out))
	assert.Contains(t, out.String(), "Usage:")
}

func TestRunUnknownCommand(t *testing.T) {
	err := Run(context.Background(), []string{"bogus"}, env(nil), io.Discard)
	assert.ErrorContains(t, err, `unknown command "bogus" for "werbot"`)
}

func TestRoleFlagsUnchanged(t *testing.T) {
	ctx := context.Background()
	assert.NoError(t, Run(ctx, []string{"gateway", "--help"}, env(nil), io.Discard))
	err := Run(ctx, []string{"gateway", "--bogus"}, env(nil), io.Discard)
	assert.ErrorContains(t, err, "flag provided but not defined: -bogus")
	err = Run(ctx, []string{"connect", "-gateway", "h", "-login", "a", "pg"}, env(nil), io.Discard)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "flag provided")
	err = Run(ctx, []string{"server", "--with-gateway"}, env(nil), io.Discard)
	assert.ErrorContains(t, err, "WERBOT_DATABASE_URL")
}

func TestRunConnectUsage(t *testing.T) {
	ctx := context.Background()
	err := Run(ctx, []string{"connect", "pg"}, env(nil), io.Discard)
	assert.ErrorContains(t, err, "WERBOT_CONNECT_GATEWAY")
	err = Run(ctx, []string{"connect"}, env(map[string]string{"WERBOT_CONNECT_GATEWAY": "gw", "WERBOT_CONNECT_LOGIN": "alice"}), io.Discard)
	assert.ErrorContains(t, err, "usage: werbot connect [flags] [<local_port>:]<resource>[:<target_port>] ...")
	assert.NoError(t, Run(ctx, []string{"connect", "-h"}, env(nil), io.Discard), "help is not an error")
}

func TestRoleRejectsExtraArgs(t *testing.T) {
	for _, role := range []string{"version", "migrate", "healthcheck", "web"} {
		err := Run(context.Background(), []string{role, "x"}, env(nil), io.Discard)
		assert.ErrorContains(t, err, "unknown command", role)
	}
}
