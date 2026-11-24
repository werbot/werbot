//go:build redteam

package redteam

import (
	"bufio"
	"context"
	"io"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/mcpserver"
	"github.com/werbot/werbot/test/cluster"
)

// Values of the approval scenarios (12 §12 item 2, ruling C7).
const (
	writeCommand    = "systemctl restart werbot-demo" // class write: runs on target only once approved
	approvalMessage = "write to prod needs approval"  // @message of the approval rules
	shortTTL        = 30 * time.Second                // lowest command_approval_ttl (settings.proto)
	defaultTTL      = 5 * time.Minute                 // command_approval_ttl of the stand
	expireWithin    = 70 * time.Second                // approval.expire runs every minute; margin 10 s
	execTimeout     = 60                              // ssh_exec timeout, seconds
	approvalWait    = 10 * time.Second                // the pending approval row appears once the gateway asks
	decisionWait    = 5 * time.Second                 // a decision releases the waiting command
)

// approvalRule makes write commands of subjects on target wait for an
// approval, refused with approvalMessage; Cleanup deletes it. It returns
// after BindingPropagation, the bound for a policy change to reach the pool.
func approvalRule(t *testing.T, name string, subjects *werbotv1.Subjects) {
	t.Helper()
	res, err := st.Edge.Policy.CreateCommandRule(context.Background(), connect.NewRequest(&werbotv1.CreateCommandRuleRequest{
		Name: name, Subjects: subjects, Classes: []string{"write"}, Effect: "approval", Message: approvalMessage,
		ResourceSelector: &werbotv1.ResourceSelector{ResourceIds: []string{st.Resources[target].String()}},
	}))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := st.Edge.Policy.DeleteCommandRule(context.Background(), connect.NewRequest(&werbotv1.DeleteCommandRuleRequest{Id: res.Msg.GetRule().GetId()}))
		assert.NoError(t, err, "delete command rule %s", name)
	})
	time.Sleep(cluster.BindingPropagation) // no gateway API exposes the policy version to poll
}

// approvalTTL sets command_approval_ttl to d; Cleanup sets defaultTTL back (ruling G25).
func approvalTTL(t *testing.T, d time.Duration) {
	t.Helper()
	set := func(d time.Duration) error {
		_, err := st.Edge.Settings.UpdateSettings(context.Background(), connect.NewRequest(&werbotv1.UpdateSettingsRequest{
			Settings:   &werbotv1.OrgSettings{CommandApprovalTtl: durationpb.New(d)},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"command_approval_ttl"}},
		}))
		return err
	}
	require.NoError(t, set(d))
	t.Cleanup(func() { assert.NoError(t, set(defaultTTL), "restore command_approval_ttl") })
}

// execResult is the outcome of an ssh_exec call made in the background.
type execResult struct {
	out mcpserver.ExecOutput
	err error
}

// execAsync calls ssh_exec of writeCommand on target with execTimeout in the background.
func execAsync(a *cluster.Agent) <-chan execResult {
	done := make(chan execResult, 1)
	go func() {
		out, err := a.Exec(context.Background(), mcpserver.ExecInput{Resource: target, Command: writeCommand, Timeout: execTimeout})
		done <- execResult{out, err}
	}()
	return done
}

// await returns the result of an execAsync call; it fails after within.
func await(t *testing.T, done <-chan execResult, within time.Duration) mcpserver.ExecOutput {
	t.Helper()
	select {
	case r := <-done:
		require.NoError(t, r.err)
		return r.out
	case <-time.After(within):
		t.Fatalf("ssh_exec did not answer within %s", within)
		return mcpserver.ExecOutput{}
	}
}

// requireWaiting requires the execAsync call done to still wait after d.
func requireWaiting(t *testing.T, done <-chan execResult, d time.Duration) {
	t.Helper()
	select {
	case r := <-done:
		t.Fatalf("the call stopped waiting: status %q, error %v", r.out.Status, r.err)
	case <-time.After(d):
	}
}

// pendingApproval waits approvalWait for the pending command approval of run.
func pendingApproval(t *testing.T, run uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.NoError(c, st.DB.QueryRow(context.Background(), `SELECT id FROM approval
			WHERE kind = 'command' AND agent_run_id = $1 AND status = 'pending' ORDER BY created_at DESC LIMIT 1`, run).Scan(&id))
	}, approvalWait, pollEvery, "pending approval of run %s", run)
	return id
}

// approvalRow is the stored state of a command approval.
type approvalRow struct {
	Kind, Status, Command string
	Classes               []string
	RunID, Instance       *uuid.UUID
	SHA256                []byte
	TTL                   time.Duration // expires_at - created_at
	Decided               bool          // decided_by is set (expiry sets decided_at only)
}

// approval reads approval id.
func approval(t require.TestingT, id uuid.UUID) approvalRow {
	var r approvalRow
	var ttl float64
	require.NoError(t, st.DB.QueryRow(context.Background(), `SELECT kind, status, coalesce(command, ''), coalesce(classes, '{}'),
		agent_run_id, gateway_instance_id, command_sha256, extract(epoch FROM expires_at - created_at)::float8,
		decided_by IS NOT NULL FROM approval WHERE id = $1`, id).
		Scan(&r.Kind, &r.Status, &r.Command, &r.Classes, &r.RunID, &r.Instance, &r.SHA256, &ttl, &r.Decided))
	r.TTL = time.Duration(ttl * float64(time.Second))
	return r
}

// waitExpired waits expireWithin for approval id to become expired.
func waitExpired(t *testing.T, id uuid.UUID) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, "expired", approval(c, id).Status)
	}, expireWithin, time.Second, "approval %s expired", id)
}

// notifyDecided sends the decision notice of approval id to the gateway
// instance that waits for it, as a decision does (ruling C7).
func notifyDecided(t *testing.T, id uuid.UUID) {
	t.Helper()
	got := approval(t, id)
	require.NotNil(t, got.Instance, "approval %s has a gateway instance", id)
	_, err := st.DB.Exec(t.Context(), "SELECT pg_notify('werbot_approval_decided', $1)", got.Instance.String()+":"+id.String())
	require.NoError(t, err)
}

// commandSessions counts the sessions that ran writeCommand: of run, or of people when run is nil.
func commandSessions(t require.TestingT, run *uuid.UUID) int {
	var n int
	require.NoError(t, st.DB.QueryRow(context.Background(),
		"SELECT count(*) FROM session WHERE command = $1 AND agent_run_id IS NOT DISTINCT FROM $2", writeCommand, run).Scan(&n))
	return n
}

// humanExec runs writeCommand on target as alice over plain SSH through the
// pool front; stderr comes line by line (CR kept) on lines, done closes when
// the session ends.
func humanExec(t *testing.T) (lines <-chan string, done <-chan struct{}) {
	t.Helper()
	sess, err := st.Dial(t, st.Addr(t, "nginx", 2222), "alice+"+target).NewSession()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sess.Close() })
	stderr, err := sess.StderrPipe()
	require.NoError(t, err)
	sess.Stdout = io.Discard
	require.NoError(t, sess.Start(writeCommand))
	out, end := make(chan string, 16), make(chan struct{})
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			out <- sc.Text()
		}
		close(out)
		_ = sess.Wait() // the command's exit status: systemctl is absent on the target
		close(end)
	}()
	return out, end
}

// nextLine returns the next stderr line of humanExec within approvalWait.
func nextLine(t *testing.T, lines <-chan string) string {
	t.Helper()
	select {
	case l, ok := <-lines:
		require.True(t, ok, "stderr closed")
		return l
	case <-time.After(approvalWait):
		t.Fatalf("no stderr line within %s", approvalWait)
		return ""
	}
}
