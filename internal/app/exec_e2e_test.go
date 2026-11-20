package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
)

func TestE2EExecDeniedWithMessage(t *testing.T) {
	s := startStack(t)
	srv := s.addResource(t, "srv", "ssh")
	require.Eventually(t, func() bool { return s.allowed(t, s.key, "alice+srv") }, settle, 100*time.Millisecond)

	s.denyDestructive(t, srv)
	ctx := context.Background()
	started := func() (n int) { // attempts before the rule reached the gateway ran
		require.NoError(t, s.pool.QueryRow(ctx, "SELECT count(*) FROM session WHERE command = 'rm -rf /'").Scan(&n))
		return n
	}
	before := started()
	_, stderr, code := s.execSplit(t, "rm -rf /")
	assert.Equal(t, "Command denied: "+denyMessage+"\r\n", stderr)
	assert.Equal(t, 1, code)
	assert.Equal(t, before, started(), "a refused command starts no session")
	stdout, _, code := s.execSplit(t, "echo hi")
	assert.Equal(t, "hi\n", stdout, "a read command still runs")
	assert.Zero(t, code)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var n int
		require.NoError(c, s.pool.QueryRow(ctx, `SELECT count(*) FROM audit_event WHERE action = 'session.deny'
			AND target_type = 'resource' AND target_id = $1 AND data->>'reason' = 'denied' AND data->>'target' = 'rm -rf /'`,
			srv.String()).Scan(&n))
		assert.Positive(c, n)
	}, 20*time.Second, 200*time.Millisecond)
}

type execResult struct {
	stdout, stderr string
	code           int
}

func TestE2EExecApprovalApproved(t *testing.T) {
	s, srv, carol := approvalStack(t)
	done := make(chan execResult, 1)
	go func() {
		stdout, stderr, code := s.execSplit(t, "hostname")
		done <- execResult{stdout, stderr, code}
	}()
	id := s.pendingCommand(t)

	ctx := context.Background()
	_, err := carol.req.Approve(ctx, connect.NewRequest(&werbotv1.ApproveRequest{Id: id}))
	require.NoError(t, err)
	var r execResult
	select {
	case r = <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the approved command did not run within 3s of the decision")
	}
	assert.Equal(t, "target\n", r.stdout)
	assert.Zero(t, r.code)
	assert.Contains(t, r.stderr, fmt.Sprintf("Waiting for approval %s, timeout 5m\r\n", id))

	var command string
	var classes []string
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT command, classes FROM session WHERE resource_id = $1 AND command IS NOT NULL
		ORDER BY started_at DESC LIMIT 1`, srv).Scan(&command, &classes))
	assert.Equal(t, "hostname", command)
	assert.Equal(t, []string{"write"}, classes, "the class rule replaces the built-in class")
	var kind string
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT data->>'kind' FROM audit_event
		WHERE action = 'access.request.approve' AND target_id = $1`, id).Scan(&kind))
	assert.Equal(t, "command", kind)
}

func TestE2EExecApprovalTimeout(t *testing.T) {
	s, _, _ := approvalStack(t)
	ctx := context.Background()
	_, err := s.pool.Exec(ctx, "UPDATE org_setting SET command_approval_ttl = '30 seconds'") // the lowest allowed
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = s.pool.Exec(ctx, "UPDATE org_setting SET command_approval_ttl = DEFAULT") })

	start := time.Now()
	stdout, stderr, code := s.execSplit(t, "hostname")
	took := time.Since(start)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, ", timeout 30s\r\n")
	assert.Contains(t, stderr, "Approval timed out\r\n")
	assert.Equal(t, 1, code)
	// the server sets the deadline by the database clock, which may run a little apart from the test's
	assert.GreaterOrEqual(t, took, 29*time.Second)
	assert.LessOrEqual(t, took, 35*time.Second)
}

func TestE2EMaskOutput(t *testing.T) {
	s := startStack(t)
	s.addResource(t, "srv", "ssh")
	ctx := context.Background()
	hc := s.aliceHTTP(t)
	_, err := werbotv1connect.NewSettingsServiceClient(hc, s.url).UpdateSettings(ctx, connect.NewRequest(&werbotv1.UpdateSettingsRequest{
		Settings: &werbotv1.OrgSettings{MaskOutput: "all"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"mask_output"}}}))
	require.NoError(t, err)

	cmd := "echo " + s.target.Password
	require.Eventually(t, func() bool {
		stdout, _, code := s.execSplit(t, cmd)
		return stdout == "[masked]\n" && code == 0
	}, settle, 100*time.Millisecond)

	var sid string
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT id::text FROM session WHERE command = $1 ORDER BY started_at DESC LIMIT 1",
		cmd).Scan(&sid))
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url+"/api/recordings/"+sid+".cast", nil)
		require.NoError(c, err)
		res, err := hc.Do(req)
		require.NoError(c, err)
		defer res.Body.Close() //nolint:errcheck // test client
		body, err := io.ReadAll(res.Body)
		require.NoError(c, err)
		require.Equal(c, http.StatusOK, res.StatusCode)
		assert.Equal(c, "true", res.Header.Get("X-Werbot-Recording-Verified"))
		assert.Contains(c, string(body), s.target.Password, "the recording keeps the original output")
	}, 20*time.Second, 200*time.Millisecond)
}
