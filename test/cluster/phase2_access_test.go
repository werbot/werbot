//go:build cluster

package cluster

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/durationpb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/clientauth"
)

// Limits of scenario 18 (spec 12 §12 #18).
const (
	GrantExpiryMax = 2 * time.Second // a shell ends this soon after its grant's expires_at
	grantTTL       = time.Minute     // the grant of target2
)

// dbSince is now() - ts on the stand's clock (Postgres, beside the gateways), not the host's.
func dbSince(t testing.TB, ts time.Time) time.Duration {
	t.Helper()
	var sec float64
	require.NoError(t, st.DB.QueryRow(context.Background(), `SELECT extract(epoch FROM now() - $1::timestamptz)::float8`, ts).Scan(&sec))
	return time.Duration(sec * float64(time.Second))
}

// TestScenario18GrantExpiresWhileReplicaIsKilled: a shell on gw1 opened under a
// one-minute grant ends within GrantExpiryMax after expires_at although server1,
// which held gw1's tunnel, was killed before: the expiry timer is local to the
// gateway (12 §12 #18). After the restore the grant no longer opens target2.
func TestScenario18GrantExpiresWhileReplicaIsKilled(t *testing.T) {
	st.Healthy(t)
	ctx := context.Background()
	pol, err := st.Edge.ApprovalPolicies.CreateApprovalPolicy(ctx, connect.NewRequest(&werbotv1.CreateApprovalPolicyRequest{
		Name: "cluster-auto-target2", Mode: "auto", Requesters: &werbotv1.Subjects{Users: []string{st.AliceID.String()}},
		ResourceSelector: &werbotv1.ResourceSelector{ResourceIds: []string{st.Resources["target2"].String()}},
	}))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := st.Edge.ApprovalPolicies.DeleteApprovalPolicy(context.Background(), connect.NewRequest(&werbotv1.DeleteApprovalPolicyRequest{Id: pol.Msg.GetApprovalPolicy().GetId()}))
		assert.NoError(t, err)
	})
	req, err := st.Edge.AccessRequests.CreateRequest(ctx, connect.NewRequest(&werbotv1.CreateRequestRequest{
		ResourceId: st.Resources["target2"].String(), Duration: durationpb.New(grantTTL),
	}))
	require.NoError(t, err)
	require.Equal(t, "approved", req.Msg.GetRequest().GetStatus(), "auto policy grants at once")
	expires := req.Msg.GetRequest().GetExpiresAt().AsTime()

	c := st.Dial(t, st.Addr(t, "nginx", 2201), "alice+target2") // gw1 -> server1
	granted := func() bool {
		out, code, err := run(c, "hostname")
		return err == nil && out == "target\n" && code == 0
	}
	require.Eventually(t, granted, BindingPropagation, 50*time.Millisecond, "the grant reaches gw1")
	sh := OpenShell(t, c)
	sh.Mark(t, "granted")
	id := st.OpenSessionOn(t, "gw1")

	t.Cleanup(func() {
		_, err := st.Compose(context.Background(), "start", "server1")
		assert.NoError(t, err)
		st.Healthy(t)
	})
	_, err = st.Compose(ctx, "kill", "server1") // down for the rest of the grant
	require.NoError(t, err)
	select {
	case <-sh.done:
	case <-time.After(GrantExpiryMax - dbSince(t, expires)):
		t.Fatalf("shell still open %s after expires_at with server1 killed; output:\n%s", GrantExpiryMax, sh.out.String())
	}
	late := dbSince(t, expires)
	t.Logf("expired under kill %s after expires_at", late)
	require.Greater(t, late, -time.Second, "the shell ended before expires_at")
	st.WaitEnded(t, id, "access_revoked", UploadDrain)

	_, err = st.Compose(ctx, "start", "server1")
	require.NoError(t, err)
	st.Healthy(t)
	out, code := Exec(t, st.Dial(t, st.Addr(t, "nginx", 2201), "alice+target2"), "hostname")
	assert.Equal(t, "Access denied", strings.TrimSpace(out), "target2 after the grant expired")
	assert.Equal(t, 1, code)
}

// TestScenario20CLILoginThroughSeparateWeb: werbot login --device against a
// werbot web replica writes a 0600 profile; ls and a certificate SSH session
// work; cli.login carries the client's IP, not one of nginx or web; logout
// revokes the CLI session and its certificate (12 §12 #20, Ruling C19).
func TestScenario20CLILoginThroughSeparateWeb(t *testing.T) {
	st.Healthy(t)
	ctx := context.Background()
	web := "http://" + st.Addr(t, "web1", 8090)
	home := cliHome(t, st.UI, web)

	var cliSession, flow, client string
	var ip *string
	require.NoError(t, st.DB.QueryRow(ctx, `SELECT target_id, data->>'flow', data->>'client_id', host(ip) FROM audit_event
		WHERE action = 'cli.login' ORDER BY created_at DESC LIMIT 1`).Scan(&cliSession, &flow, &client, &ip))
	assert.Equal(t, "device", flow)
	assert.Equal(t, "werbot-cli", client)
	require.NotNil(t, ip, "cli.login has a client IP")
	assert.NotContains(t, []string{IPNginx, IPWeb1, IPWeb2}, *ip, "audit IP is the client, not a proxy")
	seen, err := st.NewAPI(web, "").System.GetClientIP(ctx, connect.NewRequest(&werbotv1.GetClientIPRequest{}))
	require.NoError(t, err)
	assert.Equal(t, seen.Msg.GetIp(), *ip, "audit IP is this host as werbot web forwards it")

	assert.Contains(t, werbotCLI(t, home, "ls"), "target")
	p, err := clientauth.Current(home, func(string) string { return "" })
	require.NoError(t, err)
	signer, err := p.Signer(ctx, home) // issues the profile certificate
	require.NoError(t, err)
	for _, f := range []string{"profile.json", "id_ed25519", "id_ed25519-cert.pub"} {
		fi, err := os.Stat(filepath.Join(clientauth.Dir(home, p.Server), f))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm(), "mode of %s", f)
	}
	pool := st.Addr(t, "nginx", 2222)
	cfg := &gossh.ClientConfig{User: "alice+target", Auth: []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: gossh.FixedHostKey(st.HostKey), Timeout: 10 * time.Second}
	c, err := gossh.Dial("tcp", pool, cfg)
	require.NoError(t, err)
	out, code := Exec(t, c, "hostname")
	_ = c.Close()
	assert.Equal(t, "target\n", out)
	assert.Equal(t, 0, code)

	werbotCLI(t, home, "logout")
	var revoked bool
	require.NoError(t, st.DB.QueryRow(ctx, `SELECT revoked_at IS NOT NULL FROM cli_session WHERE id = $1`, cliSession).Scan(&revoked))
	assert.True(t, revoked, "logout revokes the CLI session")
	refused := func() bool {
		c, err := gossh.Dial("tcp", pool, cfg)
		if err == nil {
			_ = c.Close()
		}
		return err != nil
	}
	assert.Eventually(t, refused, BindingPropagation, 100*time.Millisecond, "the certificate of the revoked CLI session is refused")
}

// werbotCLI runs the host werbot with WERBOT_HOME=home and returns its output.
func werbotCLI(t testing.TB, home string, args ...string) string {
	t.Helper()
	cmd := exec.Command(werbotBin, args...) //nolint:gosec // host build of werbot, arguments from the test
	cmd.Env = append(os.Environ(), "WERBOT_HOME="+home)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "werbot %s:\n%s", strings.Join(args, " "), out)
	return string(out)
}
