package agentic_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/agentic"
	"github.com/werbot/werbot/internal/platform/apperr"
)

func TestLimitsValidate(t *testing.T) {
	zero := agentic.Limits{}
	require.NoError(t, zero.Validate())
	assert.Equal(t, agentic.Limits{MaxActions: 100, MaxDuration: time.Hour, MaxConcurrent: 4}, zero.WithDefaults())

	set := agentic.Limits{MaxActions: 5, MaxDuration: 10 * time.Minute, MaxConcurrent: 1}
	require.NoError(t, set.Validate())
	assert.Equal(t, set, set.WithDefaults(), "set values stay")

	for name, l := range map[string]agentic.Limits{
		"lowest":  {MaxActions: 1, MaxDuration: time.Minute, MaxConcurrent: 1},
		"highest": {MaxActions: 10000, MaxDuration: time.Hour, MaxConcurrent: 32},
	} {
		assert.NoError(t, l.Validate(), name)
	}

	for name, l := range map[string]agentic.Limits{
		"max_actions 10001":       {MaxActions: 10001},
		"max_actions negative":    {MaxActions: -1},
		"max_duration 30s":        {MaxDuration: 30 * time.Second},
		"max_duration 2h":         {MaxDuration: 2 * time.Hour},
		"max_duration negative":   {MaxDuration: -time.Second},
		"max_duration fractional": {MaxDuration: 90*time.Second + time.Millisecond},
		"max_concurrent 33":       {MaxConcurrent: 33},
		"max_concurrent negative": {MaxConcurrent: -1},
	} {
		assert.ErrorIs(t, l.Validate(), apperr.ErrInvalidArgument, name)
	}
}
