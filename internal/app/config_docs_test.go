package app

import (
	"os"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/connectcli"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/webui"
)

func TestConfigurationDocListsEveryVariable(t *testing.T) {
	raw, err := os.ReadFile("../../docs/configuration.md")
	require.NoError(t, err)
	doc := string(raw)
	vars := slices.Concat(config.Vars(), gateway.ConfigVars(), webui.ConfigVars(), connectcli.ConfigVars())
	slices.Sort(vars)
	// WERBOT_TRUSTED_PROXIES, WERBOT_LOG_LEVEL, WERBOT_LOG_FORMAT and WERBOT_METRICS_ADDR are shared by server and web.
	vars = slices.Compact(vars)
	// 25 server + 12 gateway + 2 web-only + 2 client; when the count changes, update the docs and this number.
	assert.Len(t, vars, 41)
	for _, k := range vars {
		assert.Contains(t, doc, "`"+k+"`")
	}
}
