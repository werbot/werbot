package inventory_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

var errNotAgent = errors.New("not an agent principal")

// AgentResources of a person's fake is never asked.
func (fakeAccess) AgentResources(context.Context, uuid.UUID, uuid.UUID, netip.Addr, time.Time) (map[uuid.UUID]inventory.AgentAccess, error) {
	return nil, errNotAgent
}

// agentAccess answers only for agent: its resources and classes.
type agentAccess struct {
	agent uuid.UUID
	res   map[uuid.UUID]inventory.AgentAccess
}

func (agentAccess) AccessibleResources(context.Context, uuid.UUID, uuid.UUID, netip.Addr, time.Time) ([]uuid.UUID, error) {
	return nil, errors.New("an agent principal is decided by AgentResources")
}

func (a agentAccess) AgentResources(_ context.Context, _, agentID uuid.UUID, _ netip.Addr, _ time.Time) (map[uuid.UUID]inventory.AgentAccess, error) {
	if agentID != a.agent {
		return nil, errors.New("another agent")
	}
	return a.res, nil
}

// TestListAccessibleResourcesAgent: a run token lists the resources the
// merged decision of its agent allows, each with the classes it may run alone
// or after an approval; a person's list carries no classes.
func TestListAccessibleResourcesAgent(t *testing.T) {
	e := gwSetup(t)
	allowed, hidden := e.create(t, "allowed", nil), e.create(t, "hidden", nil)
	sponsor := dbtest.User(t, e.pool, e.org, "member")
	agent, run := uuid.New(), uuid.New()
	ctx := principal.With(context.Background(), principal.Principal{UserID: sponsor, OrgID: e.org, Role: "member", AgentID: &agent, RunID: &run})
	acc := agentAccess{agent: agent, res: map[uuid.UUID]inventory.AgentAccess{
		allowed.ID: {Allowed: []string{"read"}, Approval: []string{"write"}}}}
	h := inventory.NewHandler(e.svc, fakeCreds{}, acc, e.gw, &fakeRequestable{})

	res, err := h.ListAccessibleResources(ctx, connect.NewRequest(&werbotv1.ListAccessibleResourcesRequest{}))
	require.NoError(t, err)
	require.Len(t, res.Msg.GetResources(), 1, "a resource the agent may not reach is not listed: "+hidden.Name)
	r := res.Msg.GetResources()[0]
	assert.Equal(t, allowed.ID.String(), r.GetId())
	assert.True(t, r.GetAllowed())
	assert.Equal(t, []string{"read"}, r.GetAllowedClasses())
	assert.Equal(t, []string{"write"}, r.GetApprovalClasses())

	h = inventory.NewHandler(e.svc, fakeCreds{}, fakeAccess{[]uuid.UUID{allowed.ID}}, e.gw, &fakeRequestable{})
	res, err = h.ListAccessibleResources(principal.With(context.Background(), principal.Principal{UserID: sponsor, OrgID: e.org, Role: "member"}),
		connect.NewRequest(&werbotv1.ListAccessibleResourcesRequest{}))
	require.NoError(t, err)
	require.Len(t, res.Msg.GetResources(), 1)
	assert.Empty(t, res.Msg.GetResources()[0].GetAllowedClasses(), "a person's list has no classes")
}
