//go:build redteam

package redteam

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	sshv1 "github.com/werbot/werbot/gen/werbot/connector/ssh/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

// Values of task 3 (12 §12 items 4 and 6, scenario 11b).
const (
	// revokeMax is how soon a lock, a frozen agent or a new console rule
	// ends an open session (12 §12, contract §3.2).
	revokeMax = 2 * time.Second
	// settle bounds the wait until a rule, a setting or a binding reaches
	// both gateways; it is a readiness poll, not an asserted value.
	settle = 10 * time.Second
	// reportWait bounds the wait until the gateway's session end is in the
	// database; the end time itself is checked against revokeMax.
	reportWait = 15 * time.Second
)

// allUsers and allAgents are the rule subjects of the scenarios.
var (
	allUsers  = &werbotv1.Subjects{AllUsers: true}
	allAgents = &werbotv1.Subjects{AllAgents: true}
)

// within calls cond in the test goroutine every pollEvery until it holds or
// d passes (cond may use t: unlike assert.Eventually it runs in this goroutine).
func within(d time.Duration, cond func() bool) bool {
	for end := time.Now().Add(d); ; time.Sleep(pollEvery) {
		if cond() {
			return true
		}
		if time.Now().After(end) {
			return false
		}
	}
}

// dbNow is the database clock: session ends are stamped on the stand's clock.
func dbNow(t *testing.T) time.Time {
	t.Helper()
	var now time.Time
	require.NoError(t, st.DB.QueryRow(context.Background(), "SELECT now()").Scan(&now))
	return now
}

// setMaskOutput sets org_setting.mask_output; Cleanup restores "agents" (ruling G25).
func setMaskOutput(t *testing.T, mode string) {
	t.Helper()
	update := func(mode string) error {
		_, err := st.Edge.Settings.UpdateSettings(context.Background(), connect.NewRequest(&werbotv1.UpdateSettingsRequest{
			Settings: &werbotv1.OrgSettings{MaskOutput: mode}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"mask_output"}},
		}))
		return err
	}
	t.Cleanup(func() { assert.NoError(t, update("agents"), "restore mask_output") })
	require.NoError(t, update(mode))
}

// lock creates a lock on targetType (id nil for all_agents); Cleanup deletes it.
func lock(t *testing.T, targetType string, id *uuid.UUID) uuid.UUID {
	t.Helper()
	req := &werbotv1.CreateLockRequest{TargetType: targetType, Reason: "redteam kill switch"}
	if id != nil {
		req.TargetId = id.String()
	}
	res, err := st.Edge.Locks.CreateLock(context.Background(), connect.NewRequest(req))
	require.NoError(t, err)
	lockID := uuid.MustParse(res.Msg.GetLock().GetId())
	t.Cleanup(func() { unlock(t, lockID) })
	return lockID
}

// unlock deletes a lock; an already deleted one is fine.
func unlock(t *testing.T, id uuid.UUID) {
	t.Helper()
	_, err := st.Edge.Locks.DeleteLock(context.Background(), connect.NewRequest(&werbotv1.DeleteLockRequest{Id: id.String()}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		assert.NoError(t, err, "delete lock")
	}
}

// pathRule creates a path rule of subjects on target; Cleanup deletes it.
func pathRule(t *testing.T, subjects *werbotv1.Subjects, paths []string, access, effect string) uuid.UUID {
	t.Helper()
	res, err := st.Edge.Policy.CreatePathRule(context.Background(), connect.NewRequest(&werbotv1.CreatePathRuleRequest{
		Name: "redteam-paths", Subjects: subjects, ResourceSelector: onTarget(), Paths: paths, Access: access, Effect: effect,
	}))
	require.NoError(t, err)
	id := uuid.MustParse(res.Msg.GetRule().GetId())
	t.Cleanup(func() {
		_, err := st.Edge.Policy.DeletePathRule(context.Background(), connect.NewRequest(&werbotv1.DeletePathRuleRequest{Id: id.String()}))
		assert.NoError(t, err, "delete path rule")
	})
	return id
}

// terminalRule creates a terminal rule of subjects on target; Cleanup deletes it.
func terminalRule(t *testing.T, subjects *werbotv1.Subjects, mode string) uuid.UUID {
	t.Helper()
	res, err := st.Edge.Policy.CreateTerminalRule(context.Background(), connect.NewRequest(&werbotv1.CreateTerminalRuleRequest{
		Name: "redteam-" + mode, Subjects: subjects, ResourceSelector: onTarget(), Mode: mode,
	}))
	require.NoError(t, err)
	id := uuid.MustParse(res.Msg.GetRule().GetId())
	t.Cleanup(func() { deleteTerminalRule(t, id) })
	return id
}

// deleteTerminalRule deletes a terminal rule; an already deleted one is fine.
func deleteTerminalRule(t *testing.T, id uuid.UUID) {
	t.Helper()
	_, err := st.Edge.Policy.DeleteTerminalRule(context.Background(), connect.NewRequest(&werbotv1.DeleteTerminalRuleRequest{Id: id.String()}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		assert.NoError(t, err, "delete terminal rule")
	}
}

func onTarget() *werbotv1.ResourceSelector {
	return &werbotv1.ResourceSelector{ResourceIds: []string{st.Resources[target].String()}}
}

// secretTargetID is the resource on sshtarget, created by the first useSecretTarget.
var secretTargetID uuid.UUID

// useSecretTarget creates once the resource secretTarget on sshtarget with
// the credential targetSecret, binds alice to it and waits until she reaches
// it. The resource stays for the stand's life.
func useSecretTarget(t *testing.T) {
	t.Helper()
	if secretTargetID != uuid.Nil {
		return
	}
	ctx := context.Background()
	cfg, err := anypb.New(&sshv1.Config{Host: "sshtarget", Port: 2222, Login: "werbot"})
	require.NoError(t, err)
	res, err := st.Edge.Resources.CreateResource(ctx, connect.NewRequest(&werbotv1.CreateResourceRequest{
		Name: secretTarget, Type: "ssh", GatewayId: st.PoolID.String(), Config: cfg,
	}))
	require.NoError(t, err)
	id := res.Msg.GetResource().GetId()
	_, err = st.Edge.Resources.SetCredential(ctx, connect.NewRequest(&werbotv1.SetCredentialRequest{ResourceId: id, Kind: "password", Value: []byte(targetSecret)}))
	require.NoError(t, err)
	_, err = st.Edge.Access.CreateBinding(ctx, connect.NewRequest(&werbotv1.CreateBindingRequest{SubjectType: "user", SubjectId: st.AliceID.String(), ResourceId: id}))
	require.NoError(t, err)
	require.True(t, within(settle, func() bool {
		out, code := aliceExec(t, secretTarget, "echo ready")
		return out == "ready\n" && code == 0
	}), "alice reaches %s", secretTarget)
	secretTargetID = uuid.MustParse(id)
}

// bind binds user to resource; Cleanup deletes the binding.
func bind(t *testing.T, user, resource uuid.UUID) {
	t.Helper()
	res, err := st.Edge.Access.CreateBinding(context.Background(), connect.NewRequest(&werbotv1.CreateBindingRequest{
		SubjectType: "user", SubjectId: user.String(), ResourceId: resource.String(),
	}))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := st.Edge.Access.DeleteBinding(context.Background(), connect.NewRequest(&werbotv1.DeleteBindingRequest{Id: res.Msg.GetBinding().GetId()}))
		assert.NoError(t, err, "delete binding")
	})
}

// aliceExec runs command as alice on resource through the pool front and
// returns stdout+stderr and the exit code (-1 without one). It never prints
// the output: it may carry a credential (ruling G5).
func aliceExec(t *testing.T, resource, command string) (string, int) {
	t.Helper()
	c := st.Dial(t, st.Addr(t, "nginx", 2222), "alice+"+resource)
	defer c.Close()
	sess, err := c.NewSession()
	require.NoError(t, err)
	defer sess.Close()
	out, err := sess.CombinedOutput(command)
	if ee, ok := errors.AsType[*gossh.ExitError](err); ok {
		return string(out), ee.ExitStatus()
	}
	if err != nil {
		return string(out), -1
	}
	return string(out), 0
}

// runSession waits for the open session of command in run and returns its id.
func runSession(t *testing.T, run uuid.UUID, command string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.Eventually(t, func() bool {
		return st.DB.QueryRow(context.Background(), `SELECT id FROM session
			WHERE agent_run_id = $1 AND command = $2 AND ended_at IS NULL`, run, command).Scan(&id) == nil
	}, settle, pollEvery, "open session of %q in run %s", command, run)
	return id
}

// shellSession waits for alice's newest open shell on target and returns its id.
func shellSession(t *testing.T) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.Eventually(t, func() bool {
		return st.DB.QueryRow(context.Background(), `SELECT id FROM session WHERE user_id = $1 AND resource_id = $2
			AND agent_run_id IS NULL AND command IS NULL AND ended_at IS NULL ORDER BY started_at DESC LIMIT 1`,
			st.AliceID, st.Resources[target]).Scan(&id) == nil
	}, settle, pollEvery, "open shell of alice on %s", target)
	return id
}

// open reports whether session id has not ended.
func open(t *testing.T, id uuid.UUID) bool {
	t.Helper()
	var ended bool
	require.NoError(t, st.DB.QueryRow(context.Background(), "SELECT ended_at IS NOT NULL FROM session WHERE id = $1", id).Scan(&ended))
	return !ended
}

// endedWithin requires session id to end with access_revoked at most
// revokeMax after from (database clock).
func endedWithin(t *testing.T, id uuid.UUID, from time.Time) {
	t.Helper()
	st.WaitEnded(t, id, "access_revoked", reportWait)
	var ended time.Time
	require.NoError(t, st.DB.QueryRow(context.Background(), "SELECT ended_at FROM session WHERE id = $1", id).Scan(&ended))
	t.Logf("session %s ended %s after the action", id, ended.Sub(from).Round(time.Millisecond))
	assert.LessOrEqual(t, ended.Sub(from), revokeMax, "session %s ends within %s", id, revokeMax)
}
