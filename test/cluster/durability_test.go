//go:build cluster

package cluster

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScenario10ServersDownFailClosed: with both server replicas stopped an
// active session lives on, a new one is refused as not ready within
// NotReadyMax, and the end of the active session reaches the database through
// the spool after the replicas return, with its real reason and a verified recording.
func TestScenario10ServersDownFailClosed(t *testing.T) {
	st.Healthy(t)
	t.Cleanup(func() { st.Healthy(t) }) // starts the servers if the test stops early
	ctx := context.Background()
	sh := OpenShell(t, st.Dial(t, st.Addr(t, "nginx", 2201), "alice+target"))
	sh.Mark(t, "before")
	id := st.OpenSessionOn(t, "gw1")
	gw1 := st.Addr(t, "gw1", 9090)

	t0 := time.Now()
	_, err := st.Compose(ctx, "stop", "server1", "server2")
	require.NoError(t, err)
	t.Logf("servers stopped after %s", time.Since(t0))
	sh.Mark(t, "during-outage")
	require.Eventually(t, func() bool { return Ready(gw1) == http.StatusServiceUnavailable }, 30*time.Second, 100*time.Millisecond, "gw1 /readyz without servers")
	t.Logf("gw1 not ready %s after the stop", time.Since(t0))

	t0 = time.Now() // the window includes the SSH handshake
	out, code := Exec(t, st.Dial(t, st.Addr(t, "nginx", 2201), "alice+target"), "hostname")
	took := time.Since(t0)
	assert.Contains(t, out, "Gateway is not ready, try again later")
	assert.Equal(t, 1, code)
	assert.LessOrEqual(t, took, NotReadyMax, "refusal time")
	t.Logf("refused after %s", took)

	sh.Exit(t) // EndSession and FinishRecording go to the spool
	t0 = time.Now()
	_, err = st.Compose(ctx, "start", "server1", "server2")
	require.NoError(t, err)
	require.NoError(t, poll(ctx, RestoreMax, func(ctx context.Context) error { return st.ready(ctx, "server1", "server2") }))
	t.Logf("servers ready after %s", time.Since(t0))
	t.Logf("closed after %s", st.WaitEnded(t, id, "closed", UploadDrain)) // not gateway_lost: the spool carries the real reason
	t0 = time.Now()
	st.WaitVerified(t, id, UploadDrain)
	t.Logf("verified after %s", time.Since(t0))
}

// TestScenario12S3Outage: a session that prints a mark every second lives
// through an S3Outage of SeaweedFS; after S3 returns, its recording verifies
// within UploadDrain and the replay holds every mark.
func TestScenario12S3Outage(t *testing.T) {
	st.Healthy(t)
	t.Cleanup(func() {
		_, err := st.Compose(context.Background(), "start", "--wait", "seaweedfs") // Healthy does not wait for S3
		assert.NoError(t, err)
		st.Healthy(t)
	})
	sh := OpenShell(t, st.Dial(t, st.Addr(t, "nginx", 2201), "alice+target"))
	tick := time.NewTicker(time.Second) // pacing of the output, not synchronization
	defer tick.Stop()
	n := 0
	mark := func() { // each Mark waits at most 5 s: the session is alive
		sh.Mark(t, fmt.Sprintf("tick-%03d", n))
		n++
		<-tick.C
	}
	mark() // the session row exists once the shell runs
	id := st.OpenSessionOn(t, "gw1")
	for n < 5 {
		mark()
	}

	t0 := time.Now()
	_, err := st.Compose(context.Background(), "stop", "seaweedfs")
	require.NoError(t, err)
	down := time.Now() // S3Outage counts from the stopped container: S3 may serve while it stops
	for time.Since(down) < S3Outage {
		mark()
	}
	t1 := time.Now()
	_, err = st.Compose(context.Background(), "start", "--wait", "seaweedfs")
	require.NoError(t, err)
	t.Logf("stop took %s, S3 down %s until healthy, healthy %s after start", down.Sub(t0), time.Since(down), time.Since(t1))
	for range 5 {
		mark()
	}
	sh.Exit(t)

	t.Logf("closed after %s", st.WaitEnded(t, id, "closed", UploadDrain))
	t0 = time.Now()
	st.WaitVerified(t, id, UploadDrain)
	t.Logf("verified after %s", time.Since(t0))

	verified, output := replay(t, id)
	assert.Equal(t, "true", verified, "X-Werbot-Recording-Verified")
	var missing []string
	for i := range n {
		if m := fmt.Sprintf("mark:tick-%03d\r", i); !strings.Contains(output, m) {
			missing = append(missing, m)
		}
	}
	assert.Empty(t, missing, "marks missing from the replay of %d", n)
	t.Logf("replay holds %d marks", n)
}

// replay downloads the recording of id through Edge with alice's cookie and
// returns X-Werbot-Recording-Verified and the joined output of its "o" events.
func replay(t *testing.T, id uuid.UUID) (verified, output string) {
	t.Helper()
	res, err := st.Edge.HTTP.Get(st.Edge.BaseURL + "/api/recordings/" + id.String() + ".cast")
	require.NoError(t, err)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("GET %s.cast: %s %s", id, res.Status, body)
	}
	var out strings.Builder
	sc := bufio.NewScanner(res.Body)
	sc.Buffer(nil, 1<<20)
	sc.Scan() // asciicast v2 header
	for sc.Scan() {
		var ev []any // [time, code, data]
		require.NoError(t, json.Unmarshal(sc.Bytes(), &ev), sc.Text())
		if len(ev) == 3 && ev[1] == "o" {
			s, _ := ev[2].(string)
			out.WriteString(s) // an event may split a mark: check the joined output
		}
	}
	require.NoError(t, sc.Err())
	return res.Header.Get("X-Werbot-Recording-Verified"), out.String()
}
