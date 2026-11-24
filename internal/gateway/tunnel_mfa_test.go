package gateway

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestTunnelMFACalls(t *testing.T) {
	e := newTunnelEnv(t, nil, time.Hour)
	res := uuid.New()
	key := "key:SHA256:" + fmt.Sprintf("%043d", 0)
	tk, err := e.c.CreateMFAChallenge(t.Context(), challengeRequest{UserID: testUserID, ResourceID: res, SSHSessionID: []byte{1, 2},
		ClientIP: netip.MustParseAddr("192.0.2.7"), Binding: key, Strength: "any"})
	require.NoError(t, err)
	require.Equal(t, mfaTicket{ID: "c1", Code: "BCDF-GHJK", ExpiresAt: testApprovalExpiry,
		URL: fmt.Sprintf("%s %s 0102 192.0.2.7 %s any", testUserID, res, key)}, tk)

	ok, err := e.c.VerifyMFATOTP(t.Context(), "c1", "123456")
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = e.c.VerifyMFATOTP(t.Context(), "c1", "000000")
	require.NoError(t, err)
	require.False(t, ok)
	_, err = e.c.VerifyMFATOTP(t.Context(), "c1", "999999")
	require.Equal(t, connect.CodeResourceExhausted, connect.CodeOf(err))

	status, err := e.c.GetMFAChallenge(t.Context(), "c1")
	require.NoError(t, err)
	require.Equal(t, "confirmed", status)
	_, err = e.c.GetMFAChallenge(t.Context(), uuid.NewString())
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// An mfa_confirmed wakes only the wait for its challenge and never blocks the stream.
func TestTunnelDispatchesMFAConfirmed(t *testing.T) {
	e := newTunnelEnv(t, nil, time.Hour)
	e.run(t, nil)
	require.Eventually(t, e.c.Connected, 2*time.Second, time.Millisecond)
	mine, unsubscribe := e.c.SubscribeMFA("c1")
	defer unsubscribe()
	other, unsubscribeOther := e.c.SubscribeMFA("c2")
	defer unsubscribeOther()
	e.f.push <- mfaConfirmedMsg("c1")
	e.f.push <- mfaConfirmedMsg("c1")
	e.f.push <- snapMsg(testSnap("v2"))
	require.Eventually(t, func() bool { return e.c.Policy().Version() == "v2" }, 2*time.Second, time.Millisecond)
	select {
	case <-mine:
	default:
		t.Fatal("the wait for c1 was not woken")
	}
	select {
	case <-other:
		t.Fatal("the wait for c2 was woken")
	default:
	}
	require.Equal(t, 1, e.f.attempts())
}
