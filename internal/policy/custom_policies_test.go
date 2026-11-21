package policy

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
)

func TestCustomPolicyCRUD(t *testing.T) {
	e := setup(t)
	_, err := e.svc.CreateCustomPolicy(e.ctx, e.org, CustomPolicyInput{Name: "broken", Cedar: "permit (", Enabled: true})
	require.ErrorIs(t, err, apperr.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "1:9", "the Cedar error keeps its position")
	_, err = e.svc.CreateCustomPolicy(e.ctx, e.org, CustomPolicyInput{Name: "", Cedar: approvalCedar})
	assert.ErrorIs(t, err, apperr.ErrInvalidArgument)

	in := CustomPolicyInput{Name: "review writes", Cedar: approvalCedar, Enabled: true}
	pol, err := e.svc.CreateCustomPolicy(e.ctx, e.org, in)
	require.NoError(t, err)
	assert.Equal(t, in, pol.CustomPolicyInput)
	assert.Equal(t, &e.owner, pol.CreatedBy)
	assert.False(t, pol.UpdatedAt.IsZero())
	_, err = e.svc.CreateCustomPolicy(e.ctx, e.org, in)
	assert.ErrorIs(t, err, apperr.ErrAlreadyExists)

	list, err := e.svc.ListCustomPolicies(e.ctx, e.org)
	require.NoError(t, err)
	assert.Equal(t, []CustomPolicy{pol}, list)
	snap, err := e.svc.Snapshot(e.ctx, e.org)
	require.NoError(t, err)
	assert.Equal(t, []SnapshotCustomPolicy{{ID: pol.ID, Cedar: approvalCedar}}, snap.CustomPolicies)

	in.Enabled = false
	upd, err := e.svc.UpdateCustomPolicy(e.ctx, e.org, pol.ID, in)
	require.NoError(t, err)
	assert.False(t, upd.Enabled)
	assert.False(t, upd.UpdatedAt.Before(pol.UpdatedAt))
	snap, err = e.svc.Snapshot(e.ctx, e.org)
	require.NoError(t, err)
	assert.Empty(t, snap.CustomPolicies, "disabled policies stay out of the snapshot")
	_, err = e.svc.UpdateCustomPolicy(e.ctx, e.org, uuid.New(), in)
	assert.ErrorIs(t, err, apperr.ErrNotFound)

	require.NoError(t, e.svc.DeleteCustomPolicy(e.ctx, e.org, pol.ID))
	assert.ErrorIs(t, e.svc.DeleteCustomPolicy(e.ctx, e.org, pol.ID), apperr.ErrNotFound)

	// The Cedar text never reaches the audit log.
	assert.Equal(t, []string{`{"name": "review writes", "enabled": true}`}, auditData(t, e, "custom_policy.create"))
	assert.Equal(t, []string{`{"name": "review writes", "enabled": false}`}, auditData(t, e, "custom_policy.update"))
	assert.Equal(t, []string{`{"name": "review writes", "enabled": false}`}, auditData(t, e, "custom_policy.delete"))
}
