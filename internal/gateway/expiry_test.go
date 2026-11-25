package gateway

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPolicyExpirationsAndLockMode(t *testing.T) {
	p, err := CompilePolicy(testSnap("v1")) // an older server sends neither
	require.NoError(t, err)
	require.Empty(t, p.Expirations())
	require.Equal(t, "best_effort", p.LockMode())

	s := testSnap("v2")
	s.Expirations, s.LockMode = []int64{300, 100, 200}, "strict"
	p, err = CompilePolicy(s)
	require.NoError(t, err)
	require.Equal(t, []int64{100, 200, 300}, p.Expirations())
	require.Equal(t, []int64{300, 100, 200}, s.GetExpirations(), "the snapshot stays as it came")
	p.Expirations()[0] = 0
	require.Equal(t, []int64{100, 200, 300}, p.Expirations(), "a copy")
	require.Equal(t, "strict", p.LockMode())

	var none *Policy
	require.Empty(t, none.Expirations())
	require.Equal(t, "best_effort", none.LockMode())
}

func TestNextExpiry(t *testing.T) {
	exps := []int64{100, 200}
	for _, tc := range []struct {
		name string
		now  time.Time
		want int64 // 0: none
	}{
		{"before the first", time.Unix(99, 900_000_000), 100},
		{"timer fired 5ms early: the same second again", time.Unix(100, 0).Add(-5 * time.Millisecond), 100},
		{"at the first", time.Unix(100, 0), 200},
		{"at the last", time.Unix(200, 0), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			at, ok := nextExpiry(exps, tc.now)
			require.Equal(t, tc.want != 0, ok)
			if ok {
				require.Equal(t, time.Unix(tc.want, 0), at)
			}
		})
	}
	_, ok := nextExpiry(nil, time.Unix(0, 0))
	require.False(t, ok)
}
