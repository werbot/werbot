package app

import (
	"log/slog"
	"slices"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/wrapperspb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/policy"
)

func TestCrossOrgIsolation(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	actx, orgA, userA := s.org(t, "admin")
	bctx, _, userB := s.org(t, "admin")
	ra, err := s.echo(actx, t, wrapperspb.String("a"), nil)
	require.NoError(t, err)
	rb, err := s.echo(bctx, t, wrapperspb.String("b"), nil)
	require.NoError(t, err)
	_, err = s.res.SetCredential(bctx, connect.NewRequest(&werbotv1.SetCredentialRequest{ResourceId: rb.GetId(), Kind: "password", Value: []byte("pw")}))
	require.NoError(t, err)
	s.bind(actx, t, &werbotv1.CreateBindingRequest{SubjectType: "user", SubjectId: userA.String(), ResourceId: ra.GetId()})
	bindB := s.bind(bctx, t, &werbotv1.CreateBindingRequest{SubjectType: "user", SubjectId: userB.String(), ResourceId: rb.GetId()})

	id := rb.GetId()
	for name, call := range map[string]func() error{
		"GetResource": func() error {
			_, err := s.res.GetResource(actx, connect.NewRequest(&werbotv1.GetResourceRequest{Id: id}))
			return err
		},
		"UpdateResource": func() error {
			_, err := s.res.UpdateResource(actx, connect.NewRequest(&werbotv1.UpdateResourceRequest{Id: id, Name: "x"}))
			return err
		},
		"DeleteResource": func() error {
			_, err := s.res.DeleteResource(actx, connect.NewRequest(&werbotv1.DeleteResourceRequest{Id: id}))
			return err
		},
		"SetCredential": func() error {
			_, err := s.res.SetCredential(actx, connect.NewRequest(&werbotv1.SetCredentialRequest{ResourceId: id, Kind: "password", Value: []byte("x")}))
			return err
		},
		"ClearCredential": func() error {
			_, err := s.res.ClearCredential(actx, connect.NewRequest(&werbotv1.ClearCredentialRequest{ResourceId: id}))
			return err
		},
		"ClearHostKey": func() error {
			_, err := s.res.ClearHostKey(actx, connect.NewRequest(&werbotv1.ClearHostKeyRequest{ResourceId: id}))
			return err
		},
		"CreateBinding": func() error {
			_, err := s.acc.CreateBinding(actx, connect.NewRequest(&werbotv1.CreateBindingRequest{SubjectType: "user", SubjectId: userA.String(), ResourceId: id}))
			return err
		},
		"DeleteBinding": func() error {
			_, err := s.acc.DeleteBinding(actx, connect.NewRequest(&werbotv1.DeleteBindingRequest{Id: bindB}))
			return err
		},
		"SetResourceRules": func() error {
			_, err := s.pol.SetResourceRules(actx, connect.NewRequest(&werbotv1.SetResourceRulesRequest{ResourceId: id}))
			return err
		},
		"GetResourceRules": func() error {
			_, err := s.pol.GetResourceRules(actx, connect.NewRequest(&werbotv1.GetResourceRulesRequest{ResourceId: id}))
			return err
		},
		"CheckAccess": func() error {
			_, err := s.pol.CheckAccess(actx, connect.NewRequest(&werbotv1.CheckAccessRequest{UserId: userA.String(), ResourceId: id, Ip: "10.0.0.1"}))
			return err
		},
	} {
		assert.Equal(t, connect.CodeNotFound, connect.CodeOf(call()), name)
	}

	list, err := s.res.ListResources(actx, connect.NewRequest(&werbotv1.ListResourcesRequest{}))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetResources(), 1)
	assert.Equal(t, ra.GetId(), list.Msg.GetResources()[0].GetId())
	binds, err := s.acc.ListBindings(actx, connect.NewRequest(&werbotv1.ListBindingsRequest{}))
	require.NoError(t, err)
	require.Len(t, binds.Msg.GetBindings(), 1)
	assert.Equal(t, ra.GetId(), binds.Msg.GetBindings()[0].GetResourceId())

	_, err = s.d.Credential.Reveal(actx, orgA, uuid.MustParse(id))
	assert.ErrorIs(t, err, apperr.ErrNotFound)
}

func TestLabelSelectorIsOrgScopedAndFollowsLabels(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	actx, orgA, userA := s.org(t, "admin")
	bctx, _, _ := s.org(t, "admin")
	prod := map[string]string{"env": "prod"}
	s.bind(actx, t, &werbotv1.CreateBindingRequest{SubjectType: "user", SubjectId: userA.String(), LabelSelector: prod})
	rb, err := s.echo(bctx, t, wrapperspb.String("b"), prod)
	require.NoError(t, err)

	snap, err := s.d.Policy.Snapshot(actx, orgA)
	require.NoError(t, err)
	foreign := uuid.MustParse(rb.GetId())
	assert.False(t, slices.ContainsFunc(snap.Resources, func(r policy.SnapshotResource) bool { return r.ID == foreign }))
	for _, b := range snap.Bindings {
		assert.NotContains(t, b.ResourceIDs, foreign)
	}

	ra, err := s.echo(actx, t, wrapperspb.String("a"), map[string]string{"env": "dev"})
	require.NoError(t, err)
	assert.False(t, s.allowed(actx, t, userA, ra.GetId()))
	_, err = s.res.UpdateResource(actx, connect.NewRequest(&werbotv1.UpdateResourceRequest{Id: ra.GetId(), Name: ra.GetName(), Labels: prod}))
	require.NoError(t, err)
	assert.True(t, s.allowed(actx, t, userA, ra.GetId()))
}
