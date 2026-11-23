package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/clientauth"
)

// sponsorOps makes the fake know agent "ops" of the harness sponsor.
func sponsorOps(t *testing.T, h *harness, f *agentFake) {
	t.Helper()
	_, p, err := loadAnyProfile(h.env)
	require.NoError(t, err)
	f.agents = []*werbotv1.Agent{{Id: f.agentID.String(), Name: "ops", SponsorUserId: p.UserID.String()}}
}

// fileTexts is every file under dir by relative path.
func fileTexts(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	require.NoError(t, filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path) //nolint:gosec // a test temp directory
		rel, _ := filepath.Rel(dir, path)
		out[rel] = string(raw)
		return err
	}))
	return out
}

func TestAgentProfileDirHasNoSponsorSecrets(t *testing.T) {
	h := newHarness(t)
	f := useAgentServer(t, h)
	sponsorOps(t, h, f)
	dir := filepath.Join(t.TempDir(), "agent-home")

	require.NoError(t, h.run("agents", "profile", "create", "ops", "--dir", dir, "--ttl", "48h", "--label", "ci box"))

	created := f.createdProfiles()
	require.Len(t, created, 1)
	assert.Equal(t, f.agentID.String(), created[0].AgentId)
	assert.Equal(t, "ci box", created[0].Label)
	assert.Equal(t, "48h0m0s", created[0].Ttl.AsDuration().String())

	files := fileTexts(t, dir)
	require.Len(t, files, 2, "only current and profile.json")
	var all string
	for name, text := range files {
		assert.NotContains(t, name, "id_ed25519")
		all += text
	}
	assert.Contains(t, all, profileToken)
	for _, secret := range []string{"sponsor-token", "sponsor-refresh"} {
		assert.NotContains(t, all, secret)
	}
	assert.NoDirExists(t, filepath.Join(dir, "runs"))
	assert.Contains(t, h.errOut.String(), "WERBOT_HOME="+dir+" werbot mcp")
	assert.Contains(t, h.errOut.String(), "chown -R")

	p, err := clientauth.Current(dir, func(string) string { return "" })
	require.NoError(t, err)
	assert.Equal(t, "ops", p.Agent)
	assert.Equal(t, profileToken, p.RefreshToken)
	assert.Empty(t, p.AccessToken)
}

func TestAgentProfileCreateRefusesNonEmptyDir(t *testing.T) {
	h := newHarness(t)
	f := useAgentServer(t, h)
	sponsorOps(t, h, f)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keep"), nil, 0o600))

	require.Error(t, h.run("agents", "profile", "create", "ops", "--dir", dir))
	assert.Empty(t, f.createdProfiles(), "no profile is created for a directory that is refused")
}

func TestAgentProfileCreateMakesAgent(t *testing.T) {
	h := newHarness(t)
	f := useAgentServer(t, h)
	dir := filepath.Join(t.TempDir(), "home")

	require.NoError(t, h.run("agents", "profile", "create", "fresh", "--dir", dir))
	assert.Equal(t, []string{"fresh"}, f.pick(func() []string { return f.newAgents }))
	created := f.createdProfiles()
	require.Len(t, created, 1)
	assert.Equal(t, f.agentID.String(), created[0].AgentId)
	assert.NotEmpty(t, created[0].Label, "the label defaults to the host name")
}

func TestAgentProfileListRevoke(t *testing.T) {
	h := newHarness(t)
	f := useAgentServer(t, h)
	f.pmu.Lock()
	f.profiles = []*werbotv1.AgentProfile{
		{Id: "11111111-1111-1111-1111-111111111111", AgentName: "ops", Label: "ci box", CreatedAt: timestamppb.Now()},
	}
	f.pmu.Unlock()
	require.NoError(t, h.run("agents", "profile", "list"))
	assert.Regexp(t, `ID\s+AGENT\s+LABEL\s+CREATED\s+LAST USED\s+EXPIRES`, h.out.String())
	assert.Regexp(t, `11111111-1111-1111-1111-111111111111\s+ops\s+ci box\s+\S+\s+never`, h.out.String())

	require.NoError(t, h.run("agents", "profile", "revoke", "11111111-1111-1111-1111-111111111111"))
	assert.Equal(t, []string{"11111111-1111-1111-1111-111111111111"}, f.pick(func() []string { return f.revoked }))
}

// A profile directory is an agent profile: it cannot make profiles or run human commands.
func TestAgentProfileFromAgentHomeRefused(t *testing.T) {
	h := newHarness(t)
	f := useAgentServer(t, h)
	sponsorOps(t, h, f)
	dir := filepath.Join(t.TempDir(), "home")
	require.NoError(t, h.run("agents", "profile", "create", "ops", "--dir", dir))
	hits := f.hits.Load()

	h.vars["WERBOT_HOME"] = dir
	for _, args := range [][]string{
		{"agents", "profile", "create", "ops", "--dir", filepath.Join(t.TempDir(), "x")},
		{"agents", "profile", "list"}, {"agents", "profile", "revoke", "x"}, {"agents"}, {"ls"},
	} {
		assert.ErrorIs(t, h.run(args...), clientauth.ErrAgentProfile, args)
	}
	assert.Equal(t, hits, f.hits.Load(), "no request reached the server")
	assert.Len(t, f.createdProfiles(), 1, "the refused create made no profile")
	assert.Empty(t, f.pick(func() []string { return f.revoked }))
}

// logout in a profile directory revokes the profile refresh token, not another.
func TestAgentProfileLogoutRevokesRefreshToken(t *testing.T) {
	h := newHarness(t)
	f := useAgentServer(t, h)
	sponsorOps(t, h, f)
	dir := filepath.Join(t.TempDir(), "home")
	require.NoError(t, h.run("agents", "profile", "create", "ops", "--dir", dir))

	h.vars["WERBOT_HOME"] = dir
	require.NoError(t, h.run("logout"))
	assert.Equal(t, []string{profileToken}, f.pick(func() []string { return f.revokedTokens }))
	assert.NoFileExists(t, filepath.Join(dir, "current"))
}
