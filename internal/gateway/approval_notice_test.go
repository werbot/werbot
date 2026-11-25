package gateway

import (
	"bytes"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

// The notice of an approval wait ends with one "Approved" line once the
// approval comes, so a client knows the command now runs; a refusal, a
// timeout or an approval granted before writes no such line.
func TestApprovalNoticeApprovedLine(t *testing.T) {
	const waiting = "Waiting for approval a1, timeout 5m\r\nhttps://werbot.test/requests/a1\r\n"
	for _, tc := range []struct {
		name, before, decision, want string
	}{
		{"approved after a wait", "", "approved", waiting + "Approved\r\n"},
		{"denied", "", "denied", waiting},
		{"timed out", "", "", waiting},
		{"approved before", "approved", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				e, a := newApprovalEnv(t, "")
				a.status = tc.before
				if tc.decision != "" {
					time.AfterFunc(10*time.Second, func() { a.decide(tc.decision) })
				}
				var notice bytes.Buffer
				_, _, _ = e.openWrite(t.Context(), &notice)
				require.Equal(t, tc.want, notice.String())
			})
		})
	}
}
