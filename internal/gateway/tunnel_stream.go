package gateway

import (
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// stream runs one Connect stream until it ends; got reports whether a snapshot arrived.
func (c *tunnelClient) stream(ctx context.Context, heartbeat func() *gatewayv1.Heartbeat) (got bool, err error) {
	ctx, cancel := context.WithCancelCause(ctx)
	// registered before the conn is loaded: a renewal either precedes the load or cancels this stream
	c.mu.Lock()
	c.cancelStream = cancel
	c.mu.Unlock()
	st := c.conn.Load().rpc.Connect(ctx)
	// The HTTP/2 transport ignores cancellation while it waits on the request body;
	// ending the body lets it abort the stream, which unblocks Receive and a pending Send.
	context.AfterFunc(ctx, func() { _ = st.CloseRequest() })
	var wg sync.WaitGroup
	defer func() {
		cancel(nil)
		wg.Wait()
		_ = st.CloseResponse()
		if c.connected.Load() && retryNow(err, got) { // armed before the flag drops: no not-ready gap
			c.graceEnd.Store(time.Now().Add(c.reconnectGrace).UnixNano())
		}
		c.setConnected(false)
	}()

	send := func() error {
		return st.Send(&gatewayv1.ConnectRequest{Msg: &gatewayv1.ConnectRequest_Heartbeat{Heartbeat: heartbeat()}})
	}
	// io.EOF means the server already ended the stream; Receive returns why.
	if err := send(); err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	wg.Go(func() {
		t := time.NewTicker(c.heartbeatEvery)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := send(); err != nil {
					if !errors.Is(err, io.EOF) { // on EOF the server's close error from Receive wins
						cancel(err)
					}
					return
				}
			}
		}
	})
	for {
		msg, err := st.Receive()
		if err != nil {
			if cause := context.Cause(ctx); ctx.Err() != nil && cause != nil {
				err = cause // renewal or a failed heartbeat, not the cancellation itself
			}
			return got, err
		}
		if s := msg.GetSnapshot(); s != nil {
			got = true
			c.setConnected(c.apply(s))
			c.graceEnd.Store(0) // the snapshot decides readiness now
		}
		if pi := msg.GetPoolInstances(); pi != nil {
			c.setPool(pi.GetIds())
		}
	}
}

// setPool stores the online instances of the pool; an id that does not parse is skipped.
func (c *tunnelClient) setPool(ids []string) {
	pool := make([]uuid.UUID, 0, len(ids))
	for _, s := range ids {
		if id, err := uuid.Parse(s); err == nil {
			pool = append(pool, id)
		}
	}
	c.pool.Store(&pool)
}

// poolInstances lists the online instances of the pool as the server last sent them;
// empty until it did.
func (c *tunnelClient) poolInstances() []uuid.UUID {
	if p := c.pool.Load(); p != nil {
		return *p
	}
	return nil
}

// apply compiles s unless it has the current version and reports whether the
// current Policy is s. A snapshot that does not compile is logged and the previous
// Policy stays for active sessions, but the gateway is not ready: it may miss a revoke.
func (c *tunnelClient) apply(s *gatewayv1.Snapshot) bool {
	if p := c.policy.Load(); p != nil && p.Version() == s.GetVersion() {
		return true
	}
	p, err := CompilePolicy(s)
	if err != nil {
		c.log.Error("tunnel: snapshot does not compile, not ready until one does", "version", s.GetVersion(), "error", err)
		return false
	}
	c.policy.Store(p)
	select {
	case c.changed <- struct{}{}:
	default:
	}
	return true
}

// retryNow reports a stream end that calls for an immediate reconnect: the
// certificate was renewed, or a stream that delivered a snapshot was closed for
// rebalance or shutdown. A replica that rejects new streams with "shutdown" before
// any snapshot gets the backoff, not a reconnect loop.
func retryNow(err error, got bool) bool {
	if errors.Is(err, errRenewed) {
		return true
	}
	var ce *connect.Error
	return got && errors.As(err, &ce) && ce.Code() == connect.CodeUnavailable && (ce.Message() == "rebalance" || ce.Message() == "shutdown")
}

// backoff is lo·2ⁿ capped at hi with ±20 % jitter, never below lo.
func backoff(n int, lo, hi time.Duration) time.Duration {
	d := lo
	for i := 0; i < n && d < hi; i++ {
		d *= 2
	}
	d = min(d, hi)
	j := time.Duration(float64(d) * (1 + 0.2*(2*rand.Float64()-1))) //nolint:gosec // jitter, not security
	return max(j, lo)
}
