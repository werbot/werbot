package logging_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/logging"
)

func TestNewJSONRespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	l := logging.New(&buf, "info", "json")
	l.Debug("hidden")
	l.Info("shown", "k", "v")
	assert.NotContains(t, buf.String(), "hidden")
	var rec map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &rec))
	assert.Equal(t, "shown", rec["msg"])
	assert.Equal(t, "v", rec["k"])
}
