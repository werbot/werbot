package gateway

import (
	"log/slog"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

func init() { RegisterConnector(warmConn{funcConn{typ: "warmq", kinds: []Kind{KindStream}}}) }

// warmCalls counts the Warmup calls of the "warmq" connector.
var warmCalls atomic.Int32

type warmConn struct{ funcConn }

func (warmConn) Warmup() { warmCalls.Add(1) }

// The parser warms up in the background once a snapshot holds a resource of
// a type with a Warmer; a snapshot without one starts nothing (Ruling O1).
func TestWarmupLazyOnFirstQueryResource(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		warmCalls.Store(0)
		c := &tunnelClient{changed: make(chan struct{}, 1), log: slog.New(slog.DiscardHandler)}
		snap := func(version string, types ...string) *gatewayv1.Snapshot {
			s := &gatewayv1.Snapshot{Version: version}
			for i, typ := range types {
				s.Resources = append(s.Resources, &gatewayv1.Resource{Id: uid(i).String(), Name: typ, Type: typ, ConfigJson: []byte(`""`)})
			}
			return s
		}
		require.True(t, c.apply(snap("v1", "hprobe")))
		synctest.Wait()
		require.Zero(t, warmCalls.Load(), "no type to warm up")
		require.True(t, c.apply(snap("v2", "hprobe", "warmq")))
		synctest.Wait()
		require.EqualValues(t, 1, warmCalls.Load())
	})
}
