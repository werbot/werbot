package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/cedar-policy/cedar-go"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/credential"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/tunnel"
)

// Credential markers: tests compare them only with bytes.Equal.
var (
	primaryPW = []byte("primary-pw-marker")
	agentPW   = []byte("agent-pw-marker")
)

// pg inserts the postgres resource "pg" served by the fixture pool with a
// primary and an agent_readonly credential; the agent one leaves it pending.
func (s agentStage) pg(t *testing.T) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	require.NoError(t, s.pool.QueryRow(ctx, `INSERT INTO resource (org_id, name, type, category, config, gateway_id)
		VALUES ($1, 'pg', 'postgres', 'databases', '{"host":"db","database":"shop","username":"app","secretTables":["public.secrets"]}', $2)
		RETURNING id`, s.f.OrgID, s.f.GatewayID).Scan(&id))
	require.NoError(t, s.stage4.d.Credential.Set(ctx, s.f.OrgID, id, credential.SlotPrimary, connector.CredentialPassword, primaryPW))
	require.NoError(t, s.stage4.d.Credential.Set(ctx, s.f.OrgID, id, credential.SlotAgentReadonly, connector.CredentialPassword, agentPW))
	return id
}

func (s agentStage) readonlyStatus(t *testing.T, id uuid.UUID, status string) {
	t.Helper()
	s.exec(t, "UPDATE resource SET agent_readonly_status = $2 WHERE id = $1", id, status)
}

func wantConnect(t *testing.T, err error, code connect.Code, msg string) {
	t.Helper()
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, code, ce.Code())
	assert.Equal(t, msg, ce.Message())
}

// An agent's session of a database resource gets only the agent_readonly
// credential, and only while its check is ok; pending, has_write, error and a
// missing agent credential are FailedPrecondition "agent credential not
// verified", never an empty secret. A person, and an agent on a type without a
// query action, get the primary credential.
func TestStartSessionAgentSlot(t *testing.T) {
	s := newAgentStage(t)
	ctx := context.Background()
	pg := s.pg(t)
	reveal := func(id uuid.UUID, typ string, agent bool) (tunnel.Secret, error) {
		return s.d.Credentials.Reveal(ctx, s.f.OrgID, id, typ, agent)
	}

	for _, status := range []string{"pending", "has_write", "error", "none"} {
		s.readonlyStatus(t, pg, status)
		_, err := reveal(pg, "postgres", true)
		wantConnect(t, err, connect.CodeFailedPrecondition, "agent credential not verified")
	}
	s.readonlyStatus(t, pg, "ok")
	sec, err := reveal(pg, "postgres", true)
	require.NoError(t, err)
	if !bytes.Equal(sec.Value, agentPW) {
		t.Fatal("an agent did not get the agent_readonly credential")
	}
	sec, err = reveal(pg, "postgres", false)
	require.NoError(t, err)
	if !bytes.Equal(sec.Value, primaryPW) {
		t.Fatal("a person did not get the primary credential")
	}

	s.exec(t, "DELETE FROM credential WHERE resource_id = $1 AND slot = 'agent_readonly'", pg)
	_, err = reveal(pg, "postgres", true)
	wantConnect(t, err, connect.CodeFailedPrecondition, "agent credential not verified")

	require.NoError(t, s.stage4.d.Credential.Set(ctx, s.f.OrgID, s.f.ResourceID, credential.SlotPrimary, connector.CredentialPassword, primaryPW))
	sec, err = reveal(s.f.ResourceID, "ssh", true)
	require.NoError(t, err)
	if !bytes.Equal(sec.Value, primaryPW) {
		t.Fatal("an agent on ssh did not get the primary credential")
	}
}

// The database limits of an agent's run reach AuthenticateUser with the
// defaults for the limits the agent leaves unset.
func TestAuthenticateUserAgentLimits(t *testing.T) {
	s := newAgentStage(t)
	agent, run := s.run(t, s.f.MemberID, "a1", 10)
	s.exec(t, `UPDATE agent SET limits = '{"max_rows": 50}' WHERE id = $1`, agent)
	id := s.certIdentity(t, s.f.MemberID)
	id.AgentID, id.RunID = &agent, &run
	u, err := s.d.Users.ByCert(context.Background(), s.f.OrgID, id)
	require.NoError(t, err)
	assert.Equal(t, tunnel.AgentLimits{MaxRows: 50, MaxResultBytes: 1 << 20, StatementTimeout: 30 * time.Second, MaxQueries: 1000}, u.Agent.Limits)
}

// writeEnv is an open agent session on a postgres resource and the approvals of its writes.
type writeEnv struct {
	agentStage
	pg, runID, session uuid.UUID
	gw                 tunnel.Gateway
	sha                [32]byte
}

func newWriteEnv(t *testing.T) writeEnv {
	t.Helper()
	s := newAgentStage(t)
	e := writeEnv{agentStage: s, pg: s.pg(t), sha: sha256.Sum256([]byte("UPDATE t SET a = 1"))}
	_, e.runID = s.run(t, s.f.MemberID, "a1", 10)
	e.session = e.startSession(t, &e.runID)
	e.gw = tunnel.Gateway{ID: s.f.GatewayID, InstanceID: s.f.InstanceID, OrgID: s.f.OrgID}
	return e
}

func (e writeEnv) startSession(t *testing.T, run *uuid.UUID) (id uuid.UUID) {
	t.Helper()
	require.NoError(t, e.pool.QueryRow(context.Background(), `INSERT INTO session (org_id, user_id, resource_id, gateway_id,
		gateway_instance_id, client_ip, kind, target, agent_run_id) VALUES ($1, $2, $3, $4, $5, '10.0.0.1', 'stream', 'db:5432', $6) RETURNING id`,
		e.f.OrgID, e.f.MemberID, e.pg, e.f.GatewayID, e.f.InstanceID, run).Scan(&id))
	return id
}

// approval inserts a command approval of run on resource through instance, decided ago when not pending.
func (e writeEnv) approval(t *testing.T, run, resource, instance uuid.UUID, sha []byte, status string, ago time.Duration) (id uuid.UUID) {
	t.Helper()
	require.NoError(t, e.pool.QueryRow(context.Background(), `INSERT INTO approval (org_id, kind, requester_user_id, agent_run_id,
		resource_id, command, command_sha256, status, decided_at, expires_at, gateway_instance_id)
		VALUES ($1, 'command', $2, $3, $4, 'UPDATE t SET a = 1', $5, $6::text,
		        CASE WHEN $6::text = 'pending' THEN NULL ELSE now() - make_interval(secs => $7::float8) END, now() + interval '1 hour', $8)
		RETURNING id`, e.f.OrgID, e.f.MemberID, run, resource, sha, status, ago.Seconds(), instance).Scan(&id))
	return id
}

// GetWriteCredential answers the primary credential only for an open agent
// session of the calling instance and an approved command of the same run and
// resource, sent through this instance, with the same SHA-256, decided within
// AgentApprovalReuse; every other case is PermissionDenied "write not
// approved", and the refusal never carries the credential.
func TestGetWriteCredentialBindings(t *testing.T) {
	e := newWriteEnv(t)
	ctx := context.Background()
	write := func(gw tunnel.Gateway, session, approval uuid.UUID, sha []byte) (tunnel.Secret, error) {
		return e.d.Credentials.WriteCredential(ctx, gw, session, approval, sha)
	}
	ok := e.approval(t, e.runID, e.pg, e.f.InstanceID, e.sha[:], "approved", time.Minute)
	sec, err := write(e.gw, e.session, ok, e.sha[:])
	require.NoError(t, err)
	if !bytes.Equal(sec.Value, primaryPW) {
		t.Fatal("an approved write did not get the primary credential")
	}

	_, otherRun := e.run(t, e.f.MemberID, "a2", 10)
	other := sha256.Sum256([]byte("DELETE FROM t"))
	personal := e.startSession(t, nil)
	ended := e.startSession(t, &e.runID)
	e.exec(t, "UPDATE session SET ended_at = now(), end_reason = 'closed' WHERE id = $1", ended)
	accessKind := e.approval(t, e.runID, e.pg, e.f.InstanceID, e.sha[:], "approved", time.Minute)
	e.exec(t, "UPDATE approval SET kind = 'access' WHERE id = $1", accessKind)
	foreignGW := tunnel.Gateway{ID: e.f.OtherGatewayID, InstanceID: e.f.OtherInstanceID, OrgID: e.f.OrgID}
	cases := map[string]struct {
		gw       tunnel.Gateway
		session  uuid.UUID
		approval uuid.UUID
		sha      []byte
	}{
		"another statement":        {e.gw, e.session, ok, other[:]},
		"another run":              {e.gw, e.session, e.approval(t, otherRun, e.pg, e.f.InstanceID, e.sha[:], "approved", time.Minute), e.sha[:]},
		"another resource":         {e.gw, e.session, e.approval(t, e.runID, e.f.ResourceID, e.f.InstanceID, e.sha[:], "approved", time.Minute), e.sha[:]},
		"pending":                  {e.gw, e.session, e.approval(t, e.runID, e.pg, e.f.InstanceID, e.sha[:], "pending", 0), e.sha[:]},
		"denied":                   {e.gw, e.session, e.approval(t, e.runID, e.pg, e.f.InstanceID, e.sha[:], "denied", time.Minute), e.sha[:]},
		"decided 11 min ago":       {e.gw, e.session, e.approval(t, e.runID, e.pg, e.f.InstanceID, e.sha[:], "approved", 11*time.Minute), e.sha[:]},
		"through another instance": {e.gw, e.session, e.approval(t, e.runID, e.pg, e.f.OtherInstanceID, e.sha[:], "approved", time.Minute), e.sha[:]},
		// the approval went through the caller, the session is of another instance
		"another instance's session": {foreignGW, e.session, e.approval(t, e.runID, e.pg, e.f.OtherInstanceID, e.sha[:], "approved", time.Minute), e.sha[:]},
		"another organization":       {tunnel.Gateway{ID: e.f.GatewayID, InstanceID: e.f.InstanceID, OrgID: uuid.New()}, e.session, ok, e.sha[:]},
		"a person's session":         {e.gw, personal, ok, e.sha[:]},
		"an ended session":           {e.gw, ended, ok, e.sha[:]},
		"an access approval":         {e.gw, e.session, accessKind, e.sha[:]},
		"an unknown approval":        {e.gw, e.session, uuid.New(), e.sha[:]},
		"no hash":                    {e.gw, e.session, ok, nil},
	}
	for name, c := range cases {
		_, err := write(c.gw, c.session, c.approval, c.sha)
		wantConnect(t, err, connect.CodePermissionDenied, "write not approved")
		if err != nil && bytes.Contains([]byte(err.Error()), primaryPW) {
			t.Fatal("a refusal carries the credential: " + name)
		}
	}
}

// GetProbeCredential answers the agent_readonly credential of a resource of
// the caller's pool whose status is not none, with the check version; another
// pool, another organization, status none and a resource without the agent
// credential are NotFound (never the primary credential).
func TestGetProbeCredential(t *testing.T) {
	s := newAgentStage(t)
	ctx := context.Background()
	pg := s.pg(t)
	gw := tunnel.Gateway{ID: s.f.GatewayID, InstanceID: s.f.InstanceID, OrgID: s.f.OrgID}
	sec, version, err := s.d.Credentials.ProbeCredential(ctx, gw, pg)
	require.NoError(t, err)
	if !bytes.Equal(sec.Value, agentPW) {
		t.Fatal("the probe did not get the agent_readonly credential")
	}
	var cred, res time.Time
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT c.updated_at, r.updated_at FROM resource r
		JOIN credential c ON c.resource_id = r.id AND c.slot = 'agent_readonly' WHERE r.id = $1`, pg).Scan(&cred, &res))
	assert.Equal(t, max(cred.UnixMicro(), res.UnixMicro()), version)

	s.exec(t, "UPDATE resource SET updated_at = now() + interval '1 minute' WHERE id = $1", pg)
	_, moved, err := s.d.Credentials.ProbeCredential(ctx, gw, pg)
	require.NoError(t, err)
	assert.Greater(t, moved, version, "a configuration change moves the version")

	require.NoError(t, s.stage4.d.Credential.Set(ctx, s.f.OrgID, s.f.ResourceID, credential.SlotPrimary, connector.CredentialPassword, primaryPW))
	for name, c := range map[string]struct {
		gw tunnel.Gateway
		id uuid.UUID
	}{
		"another pool":              {tunnel.Gateway{ID: s.f.OtherGatewayID, InstanceID: s.f.OtherInstanceID, OrgID: s.f.OrgID}, pg},
		"another organization":      {tunnel.Gateway{ID: s.f.GatewayID, InstanceID: s.f.InstanceID, OrgID: uuid.New()}, pg},
		"only a primary credential": {gw, s.f.ResourceID},
	} {
		_, _, err := s.d.Credentials.ProbeCredential(ctx, c.gw, c.id)
		require.ErrorIs(t, err, apperr.ErrNotFound, name)
	}
	s.readonlyStatus(t, pg, "none")
	_, _, err = s.d.Credentials.ProbeCredential(ctx, gw, pg)
	require.ErrorIs(t, err, apperr.ErrNotFound, "status none")
}

// The pool snapshot carries each resource's check status; the Cedar entity has
// agent_readonly true only at ok, so a status change changes the snapshot; the
// policy resource carries the database and secret tables of its config.
func TestSnapshotCarriesAgentReadonly(t *testing.T) {
	s := newAgentStage(t)
	ctx := context.Background()
	pg := s.pg(t)
	build := func() (status string, agentReadonly bool, raw []byte) {
		t.Helper()
		snap, err := buildSnapshot(ctx, s.d.Snapshots, s.f.OrgID, s.f.GatewayID)
		require.NoError(t, err)
		for _, r := range snap.GetResources() {
			if r.GetId() == pg.String() {
				status = r.GetAgentReadonlyStatus()
			}
		}
		var em cedar.EntityMap
		require.NoError(t, json.Unmarshal(snap.GetEntitiesJson(), &em))
		v, ok := em[cedar.NewEntityUID("Werbot::Resource", cedar.String(pg.String()))].Attributes.Get("agent_readonly")
		require.True(t, ok, "every resource entity has agent_readonly")
		return status, v == cedar.True, marshalSnapshot(t, snap)
	}
	for _, status := range []string{"pending", "has_write", "error"} {
		s.readonlyStatus(t, pg, status)
		got, ar, _ := build()
		assert.Equal(t, status, got)
		assert.False(t, ar, status)
	}
	_, _, before := build()
	s.readonlyStatus(t, pg, "ok")
	got, ar, after := build()
	assert.Equal(t, "ok", got)
	assert.True(t, ar)
	assert.NotEqual(t, before, after, "a status change changes the snapshot")

	rs, err := policyResources{s.stage4.d.Inventory}.PolicyResources(ctx, s.f.OrgID)
	require.NoError(t, err)
	for _, r := range rs {
		if r.ID == pg {
			assert.Equal(t, "shop", r.Database)
			assert.Equal(t, []string{"public.secrets"}, r.SecretTables)
			assert.True(t, r.AgentReadonly)
		} else {
			assert.False(t, r.AgentReadonly, r.Name)
		}
	}
}
