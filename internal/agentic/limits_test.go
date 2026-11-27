package agentic_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/agentic"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

func TestLimitsValidate(t *testing.T) {
	zero := agentic.Limits{}
	require.NoError(t, zero.Validate())
	assert.Equal(t, agentic.Limits{MaxActions: 100, MaxDuration: time.Hour, MaxConcurrent: 4,
		MaxRows: 1000, MaxResultBytes: 1 << 20, StatementTimeout: 30 * time.Second, MaxQueries: 1000}, zero.WithDefaults())

	set := agentic.Limits{MaxActions: 5, MaxDuration: 10 * time.Minute, MaxConcurrent: 1,
		MaxRows: 10, MaxResultBytes: 4096, StatementTimeout: 5 * time.Second, MaxQueries: 7}
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

// TestAgentDBLimits: the database limits default to 1000 rows, 1 MiB, 30s and
// 1000 queries; values out of their ranges are refused (also when stored);
// CheckRun carries the agent's limits with the defaults applied.
func TestAgentDBLimits(t *testing.T) {
	for name, l := range map[string]agentic.Limits{
		"lowest":  {MaxRows: 1, MaxResultBytes: 1024, StatementTimeout: time.Second, MaxQueries: 1},
		"highest": {MaxRows: 100000, MaxResultBytes: 64 << 20, StatementTimeout: 10 * time.Minute, MaxQueries: 100000},
	} {
		assert.NoError(t, l.Validate(), name)
	}
	for name, l := range map[string]agentic.Limits{
		"max_rows 100001":              {MaxRows: 100001},
		"max_rows negative":            {MaxRows: -1},
		"max_result_bytes 1023":        {MaxResultBytes: 1023},
		"max_result_bytes 64 MiB + 1":  {MaxResultBytes: 64<<20 + 1},
		"statement_timeout 601s":       {StatementTimeout: 601 * time.Second},
		"statement_timeout 500ms":      {StatementTimeout: 500 * time.Millisecond},
		"statement_timeout fractional": {StatementTimeout: 1500 * time.Millisecond},
		"max_queries 100001":           {MaxQueries: 100001},
		"max_queries negative":         {MaxQueries: -1},
	} {
		assert.ErrorIs(t, l.Validate(), apperr.ErrInvalidArgument, name)
	}

	svc, pool, f := newTestService(t)
	agent := dbtest.Agent(t, pool, f.OrgID, f.MemberID, "claude")
	run := dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 2)
	got, err := svc.CheckRun(bg, f.OrgID, run)
	require.NoError(t, err)
	assert.Equal(t, [4]int64{1000, 1 << 20, int64(30 * time.Second), 1000},
		[4]int64{int64(got.MaxRows), got.MaxResultBytes, int64(got.StatementTimeout), int64(got.MaxQueries)}, "no limits: the defaults")

	exec(t, pool, `UPDATE agent SET limits = '{"max_rows": 50, "max_result_bytes": 2048, "statement_timeout": 5, "max_queries": 3}' WHERE id = $1`, agent)
	got, err = svc.CheckRun(bg, f.OrgID, run)
	require.NoError(t, err)
	assert.Equal(t, [4]int64{50, 2048, int64(5 * time.Second), 3},
		[4]int64{int64(got.MaxRows), got.MaxResultBytes, int64(got.StatementTimeout), int64(got.MaxQueries)}, "the agent's limits")

	exec(t, pool, `UPDATE agent SET limits = '{"max_rows": 100001}' WHERE id = $1`, agent)
	_, err = svc.CheckRun(bg, f.OrgID, run)
	require.Error(t, err, "stored limits out of range refuse the run")
}
