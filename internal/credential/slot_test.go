package credential_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/credential"
	"github.com/werbot/werbot/internal/platform/apperr"
)

// readonlyState is resource.agent_readonly_status and _detail of e.res.
func (e env) readonlyState(t *testing.T) (status, detail string) {
	t.Helper()
	require.NoError(t, e.pool.QueryRow(context.Background(),
		"SELECT agent_readonly_status, agent_readonly_detail FROM resource WHERE id = $1", e.res).Scan(&status, &detail))
	return status, detail
}

// listenOrgChanged returns the payload of the first werbot_org_changed NOTIFY.
func listenOrgChanged(t *testing.T, pool *pgxpool.Pool) <-chan string {
	t.Helper()
	conn, err := pool.Acquire(context.Background())
	require.NoError(t, err)
	_, err = conn.Exec(context.Background(), "LISTEN werbot_org_changed")
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	out := make(chan string, 1)
	go func() {
		defer conn.Release()
		defer cancel()
		if n, err := conn.Conn().WaitForNotification(ctx); err == nil {
			out <- n.Payload
		}
		close(out)
	}()
	t.Cleanup(cancel)
	return out
}

// TestSetAgentCredentialMarksPending: Set(agent_readonly) marks the resource
// pending with an empty detail in the same transaction and notifies the
// snapshot; Clear(agent_readonly) marks it none; the primary slot never
// touches the status.
func TestSetAgentCredentialMarksPending(t *testing.T) {
	e := setup(t)
	svc := credential.New(e.pool, box(t, key(1)))
	ctx := context.Background()

	require.NoError(t, svc.Set(e.ctx, e.org, e.res, credential.SlotPrimary, connector.CredentialPassword, []byte("primary-pw")))
	status, _ := e.readonlyState(t)
	assert.Equal(t, "none", status, "the primary slot keeps none")

	_, err := e.pool.Exec(ctx, "UPDATE resource SET agent_readonly_status = 'has_write', agent_readonly_detail = 'old finding' WHERE id = $1", e.res)
	require.NoError(t, err)
	var updated time.Time
	require.NoError(t, e.pool.QueryRow(ctx, "SELECT updated_at FROM resource WHERE id = $1", e.res).Scan(&updated))

	notified := listenOrgChanged(t, e.pool)
	require.NoError(t, svc.Set(e.ctx, e.org, e.res, credential.SlotAgentReadonly, connector.CredentialPassword, []byte("agent-pw")))
	status, detail := e.readonlyState(t)
	assert.Equal(t, "pending", status)
	assert.Empty(t, detail)
	assert.Equal(t, e.org.String(), <-notified, "the snapshot of the organization is rebuilt")
	var after time.Time
	require.NoError(t, e.pool.QueryRow(ctx, "SELECT updated_at FROM resource WHERE id = $1", e.res).Scan(&after))
	assert.Equal(t, updated, after, "a status write keeps resource.updated_at")

	require.NoError(t, svc.Set(e.ctx, e.org, e.res, credential.SlotPrimary, connector.CredentialPassword, []byte("primary-pw2")))
	status, _ = e.readonlyState(t)
	assert.Equal(t, "pending", status, "the primary slot keeps the agent status")
	require.NoError(t, svc.Clear(e.ctx, e.org, e.res, credential.SlotPrimary))
	status, _ = e.readonlyState(t)
	assert.Equal(t, "pending", status, "clearing the primary slot keeps the agent status")

	require.NoError(t, svc.Clear(e.ctx, e.org, e.res, credential.SlotAgentReadonly))
	status, detail = e.readonlyState(t)
	assert.Equal(t, "none", status)
	assert.Empty(t, detail)
	_, err = svc.Reveal(e.ctx, e.org, e.res, credential.SlotAgentReadonly)
	assert.ErrorIs(t, err, apperr.ErrNotFound)
}

// TestCredentialSlotsIndependent: each slot keeps its own value and kind;
// Version is credential.updated_at in microseconds and grows on a new Set.
func TestCredentialSlotsIndependent(t *testing.T) {
	e := setup(t)
	svc := credential.New(e.pool, box(t, key(1)))
	cert := certKey(t)
	require.NoError(t, svc.Set(e.ctx, e.org, e.res, credential.SlotPrimary, connector.CredentialPassword, []byte("primary-pw")))
	require.NoError(t, svc.Set(e.ctx, e.org, e.res, credential.SlotAgentReadonly, connector.CredentialClientCert, cert))

	primary, err := svc.Reveal(e.ctx, e.org, e.res, credential.SlotPrimary)
	require.NoError(t, err)
	if primary.Kind != connector.CredentialPassword || !bytes.Equal(primary.Value, []byte("primary-pw")) {
		t.Fatal("the primary slot returned another credential")
	}
	agent, err := svc.Reveal(e.ctx, e.org, e.res, credential.SlotAgentReadonly)
	require.NoError(t, err)
	if agent.Kind != connector.CredentialClientCert || !bytes.Equal(agent.Value, cert) {
		t.Fatal("the agent_readonly slot returned another credential")
	}
	var micros int64
	require.NoError(t, e.pool.QueryRow(context.Background(),
		"SELECT (extract(epoch FROM updated_at) * 1000000)::bigint FROM credential WHERE resource_id = $1 AND slot = 'agent_readonly'",
		e.res).Scan(&micros))
	assert.Equal(t, micros, agent.Version, "Version is updated_at in microseconds")

	ids := []uuid.UUID{e.res, uuid.New()}
	kinds, err := svc.Kinds(e.ctx, e.org, ids, credential.SlotPrimary)
	require.NoError(t, err)
	assert.Equal(t, map[uuid.UUID]connector.CredentialKind{e.res: connector.CredentialPassword}, kinds)
	kinds, err = svc.Kinds(e.ctx, e.org, ids, credential.SlotAgentReadonly)
	require.NoError(t, err)
	assert.Equal(t, map[uuid.UUID]connector.CredentialKind{e.res: connector.CredentialClientCert}, kinds)

	require.NoError(t, svc.Set(e.ctx, e.org, e.res, credential.SlotAgentReadonly, connector.CredentialPassword, []byte("agent-pw2")))
	again, err := svc.Reveal(e.ctx, e.org, e.res, credential.SlotAgentReadonly)
	require.NoError(t, err)
	assert.Greater(t, again.Version, agent.Version, "a new Set raises the version")
	if !bytes.Equal(again.Value, []byte("agent-pw2")) {
		t.Fatal("the agent_readonly slot kept the old value")
	}
	primary, err = svc.Reveal(e.ctx, e.org, e.res, credential.SlotPrimary)
	require.NoError(t, err)
	if !bytes.Equal(primary.Value, []byte("primary-pw")) {
		t.Fatal("a Set of the agent slot changed the primary slot")
	}

	require.NoError(t, svc.Clear(e.ctx, e.org, e.res, credential.SlotAgentReadonly))
	primary, err = svc.Reveal(e.ctx, e.org, e.res, credential.SlotPrimary)
	require.NoError(t, err, "Clear of the agent slot keeps the primary slot")
	assert.Equal(t, connector.CredentialPassword, primary.Kind)
}

// TestSlotUnknownIsInvalid: a slot other than primary and agent_readonly is
// refused before any write or read.
func TestSlotUnknownIsInvalid(t *testing.T) {
	e := setup(t)
	svc := credential.New(e.pool, box(t, key(1)))
	assert.ErrorIs(t, svc.Set(e.ctx, e.org, e.res, "other", connector.CredentialPassword, []byte("pw")), apperr.ErrInvalidArgument)
	assert.ErrorIs(t, svc.Set(e.ctx, e.org, e.res, "", connector.CredentialPassword, []byte("pw")), apperr.ErrInvalidArgument)
	assert.ErrorIs(t, svc.Clear(e.ctx, e.org, e.res, "other"), apperr.ErrInvalidArgument)
	_, err := svc.Reveal(e.ctx, e.org, e.res, "other")
	assert.ErrorIs(t, err, apperr.ErrInvalidArgument)
	_, err = svc.Kinds(e.ctx, e.org, []uuid.UUID{e.res}, "other")
	assert.ErrorIs(t, err, apperr.ErrInvalidArgument)
}
