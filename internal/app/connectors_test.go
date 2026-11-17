package app

import (
	"log/slog"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

func TestSetCredentialRejectsTCP(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	ctx, _, _ := s.org(t, "owner")
	r, err := s.res.CreateResource(ctx, connect.NewRequest(&werbotv1.CreateResourceRequest{
		Name: "tcp1", Type: "tcp", ConfigJson: `{"host":"db","ports":[{"port":5432}]}`}))
	require.NoError(t, err)

	_, err = s.res.SetCredential(ctx, connect.NewRequest(&werbotv1.SetCredentialRequest{
		ResourceId: r.Msg.GetResource().GetId(), Kind: "password", Value: []byte("x")}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
