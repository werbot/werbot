package connectcli

import (
	"context"
	"io"
	"sync/atomic"
	"time"
)

// proxyDrain is how long Proxy waits for the gateway's end after the end of
// its input on a connection without a write half-close; tests shorten it.
var proxyDrain = 5 * time.Second

// Proxy dials ep and copies in to the connection and the connection to out
// until the connection ends (nil) or ctx ends (nil). At the end of in the
// connection's write half is closed (TCP); a WebSocket connection, which has
// none, is closed after proxyDrain unless the gateway ends it first.
// ponytail: a read of in pending when Proxy returns ends with in's next data or
// end; in is the process's stdin, which ends with the process.
func Proxy(ctx context.Context, ep Endpoint, in io.Reader, out io.Writer) error {
	conn, err := Dial(ctx, ep)
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // the error is in the copy

	var ours atomic.Bool // the connection was closed here, not by the gateway
	stop := context.AfterFunc(ctx, func() { ours.Store(true); _ = conn.Close() })
	defer stop()
	go func() {
		_, _ = io.Copy(conn, in)
		if cw, ok := conn.(interface{ CloseWrite() error }); ok {
			_ = cw.CloseWrite()
			return
		}
		time.AfterFunc(proxyDrain, func() { ours.Store(true); _ = conn.Close() })
	}()
	_, err = io.Copy(out, conn)
	if ours.Load() {
		return nil
	}
	return err
}
