package gateway

import (
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"sync"
	"time"

	"connectrpc.com/connect"

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
					cancel(err)
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
			c.apply(s)
			got = true
			c.setConnected(true)
		}
	}
}

// apply compiles s unless it has the current version; a snapshot that does not
// compile is logged and the previous one stays.
func (c *tunnelClient) apply(s *gatewayv1.Snapshot) {
	if p := c.policy.Load(); p != nil && p.Version() == s.GetVersion() {
		return
	}
	p, err := CompilePolicy(s)
	if err != nil {
		c.log.Error("tunnel: snapshot does not compile, keeping the previous one", "version", s.GetVersion(), "error", err)
		return
	}
	c.policy.Store(p)
}

// retryNow reports a stream end that calls for an immediate reconnect: the server
// rebalances or shuts down, or the certificate was renewed.
func retryNow(err error) bool {
	if errors.Is(err, errRenewed) {
		return true
	}
	var ce *connect.Error
	return errors.As(err, &ce) && ce.Code() == connect.CodeUnavailable && (ce.Message() == "rebalance" || ce.Message() == "shutdown")
}

// backoff is backoffMin·2ⁿ capped at backoffMax with ±20 % jitter, never below backoffMin.
func (c *tunnelClient) backoff(n int) time.Duration {
	d := c.backoffMin
	for i := 0; i < n && d < c.backoffMax; i++ {
		d *= 2
	}
	d = min(d, c.backoffMax)
	j := time.Duration(float64(d) * (1 + 0.2*(2*rand.Float64()-1))) //nolint:gosec // jitter, not security
	return max(j, c.backoffMin)
}
