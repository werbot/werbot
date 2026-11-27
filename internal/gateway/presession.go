package gateway

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"time"
)

// Limits of a PreSession.
const (
	PreSessionTimeout = 10 * time.Second // the client must start within it
	PreSessionBuffer  = 16 << 10         // the most peek can see
)

// ErrHandled is the Open error of a stream its PreSession served (a cancel
// request): no session and no denial.
var ErrHandled = errors.New("gateway: handled before session")

// errPreSessionTimeout refuses a stream whose client did not start within PreSessionTimeout.
var errPreSessionTimeout = errors.New("gateway: no session start within the pre-session timeout")

// preSession runs the PreSession of res's connector on the stream r.Accept
// accepts, for a KindStream request whose front accepts early. It returns args
// with the database the PreSession chose and the stream that replays the
// peeked bytes; a refusal closes the stream. A request it does not apply to
// returns args and no stream; a front that cannot accept early is refused.
func (p *Pipeline) preSession(ctx context.Context, r OpenRequest, res Resource, args startArgs) (startArgs, io.ReadWriteCloser, error) {
	c, _ := LookupConnector(res.Type)
	ps, ok := c.(PreSession)
	if !ok || r.Kind != KindStream {
		return args, nil, nil
	}
	if r.Accept == nil { // the checks of the PreSession never run: refuse
		return args, nil, fmt.Errorf("%w: stream front cannot run the pre-session", ErrDenied)
	}
	st, err := r.Accept()
	if err != nil {
		return args, nil, err
	}
	br := bufio.NewReaderSize(st, PreSessionBuffer)
	pctx, cancel := context.WithTimeout(ctx, PreSessionTimeout)
	defer cancel()
	stop := context.AfterFunc(pctx, func() { _ = st.Close() })
	out, err := p.runPreSession(pctx, ps, res, br.Peek, st)
	// stop before cancel: the stream outlives pctx once it is kept (Ruling C20)
	if err = preSessionErr(pctx, stop(), out, err); err != nil {
		_ = st.Close()
		return args, nil, err
	}
	args.DBName = out.DBName
	return args, peeked(br, st), nil
}

// runPreSession calls ps; a panic refuses only this stream.
func (p *Pipeline) runPreSession(ctx context.Context, ps PreSession, res Resource, peek func(int) ([]byte, error), st io.ReadWriteCloser) (out PreSessionResult, err error) {
	defer func() {
		if v := recover(); v != nil {
			p.panics.Inc()
			p.log.Error("session: connector pre-session panic", "type", res.Type, "panic", fmt.Sprint(v), "stack", string(debug.Stack()))
			err = fmt.Errorf("%w: %s", errConnectorPanic, res.Type)
		}
	}()
	return ps.PreSession(ctx, res, peek, st)
}

// preSessionErr is the Open error of a PreSession that returned out and err;
// kept reports that its timeout had not closed the stream.
func preSessionErr(pctx context.Context, kept bool, out PreSessionResult, err error) error {
	switch {
	case !kept && errors.Is(pctx.Err(), context.DeadlineExceeded):
		return errPreSessionTimeout
	case err != nil:
		return err
	case !kept: // the request ended: the client is gone
		return context.Cause(pctx)
	case out.Handled:
		return ErrHandled
	}
	return nil
}

// replayStream reads the bytes a PreSession peeked before the rest of the
// stream; replayStreamCW keeps CloseWrite for a stream that has it.
type replayStream struct {
	io.Reader
	io.WriteCloser
}

type replayStreamCW struct {
	replayStream
	closeWriter
}

func peeked(br *bufio.Reader, st io.ReadWriteCloser) io.ReadWriteCloser {
	rs := replayStream{br, st}
	if cw, ok := st.(closeWriter); ok {
		return replayStreamCW{rs, cw}
	}
	return rs
}
