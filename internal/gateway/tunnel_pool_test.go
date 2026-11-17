package gateway

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

func TestTunnelReceivesPoolInstances(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	e := newTunnelEnv(t, func(int) attempt {
		return attempt{snap: testSnap("v1"), pool: []string{a.String(), "not-a-uuid", b.String()}}
	}, time.Hour)
	require.Empty(t, e.c.poolInstances(), "none until the server sent them")
	e.run(t, func() *gatewayv1.Heartbeat { return &gatewayv1.Heartbeat{} })
	require.Eventually(t, func() bool { return len(e.c.poolInstances()) == 2 }, 2*time.Second, 5*time.Millisecond)
	require.Equal(t, []uuid.UUID{a, b}, e.c.poolInstances())
}
