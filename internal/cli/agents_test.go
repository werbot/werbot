package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/clientauth"
)

func TestAgentsCommand(t *testing.T) {
	h := newHarness(t)
	f := useAgentServer(t, h)
	f.agents = []*werbotv1.Agent{
		{Name: "ops", Status: "active", ActiveRuns: 2, Limits: &werbotv1.AgentLimits{MaxActions: 50, MaxDuration: durationpb.New(30 * time.Minute), MaxConcurrent: 2}},
		{Name: "bare", Status: "frozen"},
	}
	require.NoError(t, h.run("agents"))
	out := h.out.String()
	assert.Regexp(t, `NAME\s+STATUS\s+MAX ACTIONS\s+MAX DURATION\s+MAX CONCURRENT\s+ACTIVE RUNS`, out)
	assert.Regexp(t, `ops\s+active\s+50\s+30m0s\s+2\s+2`, out)
	assert.Regexp(t, `bare\s+frozen\s+default\s+default\s+default\s+0`, out, "unset limits say so")

	h.logout(t)
	assert.ErrorIs(t, h.run("agents"), clientauth.ErrNotLoggedIn)
}
