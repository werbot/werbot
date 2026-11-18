//go:build load

package load

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/test/cluster"
)

const (
	drainMax       = 5 * time.Minute // L1: spools empty and every session ended (ruling R8)
	verifyParallel = 32
)

// settled is the end state after a run: Took is how long the spools took to
// empty with every session of the run ended; OK is false when drainMax passed first.
type settled struct {
	Took     time.Duration
	OK       bool
	Spool    [2]int64 // werbot_gateway_spool_bytes of gw1, gw2 at the last poll
	Sessions []*werbotv1.Session
	Ended    int
	LastErr  error
	Want     int
}

func (s settled) String() string {
	return fmt.Sprintf("spool gw1=%d gw2=%d, %d of %d sessions listed, %d ended, last error %v", s.Spool[0], s.Spool[1], len(s.Sessions), s.Want, s.Ended, s.LastErr)
}

// settle polls every 5 s, up to drainMax, until both gateway spools are empty
// and ListSessions (resource, from) returns want sessions, all ended.
func settle(t testing.TB, st *cluster.Stand, resource uuid.UUID, from time.Time, want int) settled {
	t.Helper()
	gw := [2]string{st.Addr(t, "gw1", 9090), st.Addr(t, "gw2", 9090)}
	s := settled{Want: want}
	start := time.Now()
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		s.LastErr = nil
		for i, addr := range gw {
			if v, err := cluster.ReadMetric(addr, "werbot_gateway_spool_bytes"); err != nil {
				s.LastErr = err
			} else {
				s.Spool[i] = v
			}
		}
		if list, err := listSessions(context.Background(), st, resource, from); err != nil {
			s.LastErr = err
		} else {
			s.Sessions, s.Ended = list, 0
			for _, x := range list {
				if x.GetEndedAt() != nil {
					s.Ended++
				}
			}
		}
		s.Took = time.Since(start)
		if s.LastErr == nil && s.Spool == [2]int64{} && len(s.Sessions) == want && s.Ended == want {
			s.OK = true
			return s
		}
		if s.Took >= drainMax {
			return s
		}
		<-tick.C
	}
}

// listSessions reads every page (200 per page, ruling G7).
func listSessions(ctx context.Context, st *cluster.Stand, resource uuid.UUID, from time.Time) ([]*werbotv1.Session, error) {
	var all []*werbotv1.Session
	for page := int32(1); page != 0; {
		res, err := st.Edge.Sessions.ListSessions(ctx, connect.NewRequest(&werbotv1.ListSessionsRequest{
			ResourceId: resource.String(), From: timestamppb.New(from), Page: page, PageSize: 200,
		}))
		if err != nil {
			return nil, fmt.Errorf("list sessions page %d: %w", page, err)
		}
		all = append(all, res.Msg.GetSessions()...)
		page = res.Msg.GetNextPage()
	}
	return all, nil
}

// verifyAll calls VerifyRecording for every session, verifyParallel at a time,
// returns how many verified and logs the first failures.
func verifyAll(t testing.TB, st *cluster.Stand, sessions []*werbotv1.Session) int {
	t.Helper()
	var (
		mu       sync.Mutex
		verified int
		fails    []string
		wg       sync.WaitGroup
		sem      = make(chan struct{}, verifyParallel)
	)
	for _, s := range sessions {
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			res, err := st.Edge.Sessions.VerifyRecording(context.Background(), connect.NewRequest(&werbotv1.VerifyRecordingRequest{SessionId: s.GetId()}))
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err != nil:
				fails = append(fails, s.GetId()+": "+err.Error())
			case !res.Msg.GetVerified():
				fails = append(fails, s.GetId()+": "+res.Msg.GetReason())
			default:
				verified++
			}
		})
	}
	wg.Wait()
	if len(fails) > 0 {
		t.Logf("%d recordings not verified, first: %s", len(fails), strings.Join(fails[:min(len(fails), 10)], "; "))
	}
	return verified
}

// reportRSS reports the sample with the highest RSS of service with its budget
// at that moment; it passes when no sample is over its own budget.
func reportRSS(t testing.TB, load string, samples []sample, service string, budget func(int64) int64) {
	t.Helper()
	var top sample
	n, over := 0, 0
	for _, s := range samples {
		rss, ok := s.RSS[service]
		if !ok {
			continue
		}
		n++
		if rss > budget(s.Active[service]) {
			over++
		}
		if rss > top.RSS[service] {
			top = s
		}
	}
	if n == 0 {
		report(t, load, "RSS "+service, "≤ budget", "no samples", false)
		return
	}
	a := top.Active[service]
	report(t, load, "RSS "+service, "≤ budget",
		fmt.Sprintf("max %s, budget %s at %d active; %d of %d samples over", mb(top.RSS[service]), mb(budget(a)), a, over, n), over == 0)
}

// reportPG reports the most Postgres connections of each server replica in the samples.
func reportPG(t testing.TB, load string, samples []sample) {
	t.Helper()
	var top [2]int64
	for _, s := range samples {
		top[0], top[1] = max(top[0], s.PGConns["server1"]), max(top[1], s.PGConns["server2"])
	}
	limit := int64(cluster.PoolMaxConns + 2)
	report(t, load, "Postgres connections server1 / server2", fmt.Sprintf("≤ %d", limit),
		fmt.Sprintf("max %d / %d", top[0], top[1]), top[0] <= limit && top[1] <= limit)
}

// reportSlow counts statements of 250 ms or longer in the Postgres log (ruling R18, reference).
func reportSlow(t testing.TB, load string, st *cluster.Stand) {
	t.Helper()
	out, err := st.Compose(context.Background(), "logs", "--no-color", "postgres")
	if err != nil {
		report(t, load, "slow statements (≥ 250 ms)", "reference", err.Error(), false)
		return
	}
	report(t, load, "slow statements (≥ 250 ms)", "reference", fmt.Sprint(strings.Count(out, "duration:")), true)
}
