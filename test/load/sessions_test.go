//go:build load

package load

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/test/cluster"
)

const (
	readyTimeout = 30 * time.Second // dial, SSH handshake, shell and the target's "ready"
	outputGap    = 10 * time.Second // with output: a longer silence is a session error
	exitWait     = 10 * time.Second // after "exit": wait for the session to close
)

// result is one SSH session of runSessions. Connect is set once the target
// printed "ready"; Err is a connection error, a close before hold, or (with
// output) no data for longer than outputGap.
type result struct {
	Connect time.Duration
	Err     error
	Bytes   int64
}

// runSessions starts n shell sessions as alice+load-target, rate per second,
// over addrs in turn; each holds for hold after "ready", then types exit.
func runSessions(ctx context.Context, t testing.TB, st *cluster.Stand, addrs []string, n, rate int, hold time.Duration, output bool) []result {
	t.Helper()
	res := make([]result, n)
	var wg sync.WaitGroup
	tick := time.NewTicker(time.Second / time.Duration(max(rate, 1)))
	defer tick.Stop()
	start := time.Now()
	for i := range n {
		if i > 0 {
			select {
			case <-ctx.Done():
				res[i].Err = ctx.Err()
				continue
			case <-tick.C:
			}
		}
		wg.Go(func() { res[i] = session(ctx, st, addrs[i%len(addrs)], hold, output) })
	}
	t.Logf("%d sessions started in %s", n, time.Since(start).Round(time.Second))
	wg.Wait()
	return res
}

// session runs one shell session through the gateway at addr.
func session(ctx context.Context, st *cluster.Stand, addr string, hold time.Duration, output bool) (r result) {
	start := time.Now()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		r.Err = fmt.Errorf("dial: %w", err)
		return r
	}
	defer conn.Close()
	_ = conn.SetDeadline(start.Add(readyTimeout)) // bounds the handshake, the shell and "ready"
	cc, chans, reqs, err := gossh.NewClientConn(conn, addr, &gossh.ClientConfig{
		User: "alice+load-target", Auth: []gossh.AuthMethod{gossh.PublicKeys(st.Key)}, HostKeyCallback: gossh.FixedHostKey(st.HostKey),
	})
	if err != nil {
		r.Err = fmt.Errorf("handshake: %w", err)
		return r
	}
	c := gossh.NewClient(cc, chans, reqs)
	defer c.Close()
	in, out, err := openShell(c)
	if err != nil {
		r.Err = err
		return r
	}

	done := make(chan struct{})
	defer close(done)
	reads := make(chan []byte) // closed at the end of the output; readErr is set before
	var readErr error
	go func() {
		defer close(reads)
		buf := make([]byte, 32*1024)
		for {
			n, err := out.Read(buf)
			if n > 0 {
				select {
				case reads <- slices.Clone(buf[:n]):
				case <-done:
					return
				}
			}
			if err != nil {
				readErr = err
				return
			}
		}
	}()

	for seen := []byte{}; !bytes.Contains(seen, []byte("ready")); {
		b, ok := <-reads // the connection deadline ends this wait
		if !ok {
			r.Err = fmt.Errorf("closed before ready: %w", readErr)
			return r
		}
		seen = append(seen, b...)
		r.Bytes += int64(len(b))
	}
	r.Connect = time.Since(start)
	_ = conn.SetDeadline(time.Time{})

	holdEnd := time.NewTimer(hold)
	defer holdEnd.Stop()
	gap := time.NewTimer(outputGap)
	defer gap.Stop()
	if !output {
		gap.Stop()
	}
	ready := time.Now()
	for held := false; !held; {
		select {
		case b, ok := <-reads:
			if !ok {
				r.Err = fmt.Errorf("closed %s after ready, before hold %s: %w", time.Since(ready).Round(time.Second), hold, readErr)
				return r
			}
			r.Bytes += int64(len(b))
			if output {
				gap.Reset(outputGap)
			}
		case <-gap.C:
			r.Err = fmt.Errorf("no output for %s, %s after ready", outputGap, time.Since(ready).Round(time.Second))
			return r
		case <-ctx.Done():
			r.Err = ctx.Err()
			return r
		case <-holdEnd.C:
			held = true
		}
	}

	if _, err := io.WriteString(in, "exit\r"); err != nil {
		r.Err = fmt.Errorf("exit: %w", err)
		return r
	}
	wait := time.NewTimer(exitWait)
	defer wait.Stop()
	for {
		select {
		case b, ok := <-reads:
			if !ok {
				return r
			}
			r.Bytes += int64(len(b))
		case <-wait.C:
			return r // the session list check sees a session that did not end
		}
	}
}

// openShell starts a shell on an xterm 80x24 PTY and returns its stdin and stdout.
func openShell(c *gossh.Client) (io.Writer, io.Reader, error) {
	sess, err := c.NewSession()
	if err != nil {
		return nil, nil, fmt.Errorf("session: %w", err)
	}
	if err := sess.RequestPty("xterm", 24, 80, gossh.TerminalModes{}); err != nil {
		return nil, nil, fmt.Errorf("pty: %w", err)
	}
	in, err := sess.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	out, err := sess.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := sess.Shell(); err != nil {
		return nil, nil, fmt.Errorf("shell: %w", err)
	}
	return in, out, nil
}

// waitTarget waits until a session through every addr reaches the target: the
// binding of load-target has reached both instances.
func waitTarget(t testing.TB, addrs ...string) {
	t.Helper()
	var last error
	var mu sync.Mutex
	reached := func() bool {
		for _, addr := range addrs {
			if r := session(context.Background(), st, addr, 0, false); r.Err != nil {
				mu.Lock()
				last = fmt.Errorf("%s: %w", addr, r.Err)
				mu.Unlock()
				return false
			}
		}
		return true
	}
	if !assert.Eventually(t, reached, time.Minute, time.Second) {
		mu.Lock()
		defer mu.Unlock()
		t.Fatalf("load-target not reachable: %v", last)
	}
}

// failed counts the session errors and logs the first ten.
func failed(t testing.TB, res []result) int {
	t.Helper()
	var errs []error
	for _, r := range res {
		if r.Err != nil {
			errs = append(errs, r.Err)
		}
	}
	if len(errs) > 0 {
		t.Logf("%d session errors, first: %v", len(errs), errors.Join(errs[:min(len(errs), 10)]...))
	}
	return len(errs)
}

// connects are the connect times of the sessions that reached "ready".
func connects(res []result) []time.Duration {
	var d []time.Duration
	for _, r := range res {
		if r.Connect > 0 {
			d = append(d, r.Connect)
		}
	}
	return d
}
