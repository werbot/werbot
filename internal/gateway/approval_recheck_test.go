package gateway

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
)

// approvalPolicyAt is approvalPolicies("") of version v2 with resource x
// named name, on host, with idle timeout idle.
func approvalPolicyAt(t *testing.T, m ids, name, host string, idle time.Duration) *Policy {
	t.Helper()
	s := testSnapshot(t, m, approvalPolicies(""), nil, res{"rx", name, "execy"}, res{"rfake", "fake", "fake"}, res{"rssh", "ssh", "ssh"})
	s.Version = "v2"
	s.Resources[0].ConfigJson = []byte(`{"host":"` + host + `"}`)
	s.Resources[0].IdleTimeout = durationpb.New(idle)
	p, err := CompilePolicy(s)
	require.NoError(t, err)
	return p
}

// A resource whose endpoint changed during the wait refuses the approved
// command: no session starts, so no credential reaches either host.
func TestApprovedButResourceMoved(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, a := newApprovalEnv(t, "")
		moved := approvalPolicyAt(t, e.m, "x", "10.0.0.6", 0)
		time.AfterFunc(10*time.Second, func() { e.f.setPolicy(moved); a.decide("approved") })
		_, _, err := e.openWrite(t.Context(), nil)
		require.ErrorIs(t, err, ErrDenied)
		require.Equal(t, "Access denied", openText(err, "x", KindExec, nil))
		require.Zero(t, e.startCalls())
		e.requireDenial(t, "denied")
	})
}

// An approved command runs on the resource as the snapshot of the recheck
// lists it, with its limits, and the sweeper starts from that snapshot.
func TestApprovedUsesCurrentResource(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, a := newApprovalEnv(t, "")
		renamed := approvalPolicyAt(t, e.m, "x2", "10.0.0.5", 7*time.Minute)
		time.AfterFunc(10*time.Second, func() { e.f.setPolicy(renamed); a.decide("approved") })
		s, _, err := e.openWrite(t.Context(), nil)
		require.NoError(t, err)
		require.Equal(t, "x2", s.Resource.Name)
		require.Equal(t, 7*time.Minute, s.Resource.IdleTimeout)
		e.p.active.mu.Lock()
		tr := *e.p.active.sessions[s.ID]
		e.p.active.mu.Unlock()
		require.Equal(t, "x2", tr.req.Resource.Name)
		require.Equal(t, 7*time.Minute, tr.idle)
		require.Equal(t, "v2", tr.version, "decided on v2: the sweeper does not re-decide it")
	})
}
