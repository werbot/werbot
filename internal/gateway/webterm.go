package gateway

import (
	"context"
	"errors"
	"io"
)

// causeLost ends a web session whose stream broke without a close frame
// (a server replica died).
const causeLost endCause = "error"

// serveTerminal opens r on st and runs it until it ends; a browser close
// cancels the open too. An Open error is written as on SSH and closes st by
// its kind; a connector error or panic after Run closes it with "error"; any
// other end, a gateway cut included, "closed".
func serveTerminal(ctx context.Context, p *Pipeline, st relayStream, r OpenRequest) {
	term := newWebTerminal(st)
	r.Notice = term
	sctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	go term.pump(cancel)
	defer term.Close()
	s, err := p.Open(sctx, r)
	if err != nil {
		say(term, openText(err, r.Resource, r.Kind, p.ctl.Policy()))
		_ = st.Close(streamCloseReason(err))
		return
	}
	c, ok := LookupConnector(s.Resource.Type)
	if !ok { // Open found it; registrations are never removed
		p.abort(s)
		say(term, openText(ErrUnsupportedType, r.Resource, r.Kind, p.ctl.Policy()))
		_ = st.Close("denied")
		return
	}
	s.Terminal = &Terminal{ReadWriter: term, Term: "xterm-256color", Window: r.Window, Resize: term.resize, SetExitStatus: func(int) {}}
	if err := p.Run(sctx, s, c); err != nil && (errors.Is(err, errConnectorPanic) || !s.cut && sctx.Err() == nil) {
		say(term, runText(err, r.Resource))
		_ = st.Close("error")
		return
	}
	_ = st.Close("closed")
}

// webTerminal is the terminal of a web session: output goes to the stream,
// input and window changes come from it through pump. Closing it ends the
// input once the session is over: a pump blocked on it returns.
type webTerminal struct {
	*io.PipeReader
	st     relayStream
	inW    *io.PipeWriter
	resize chan Window
}

func newWebTerminal(st relayStream) *webTerminal {
	r, w := io.Pipe()
	return &webTerminal{PipeReader: r, st: st, inW: w, resize: make(chan Window, 1)}
}

func (t *webTerminal) Write(p []byte) (int, error) {
	if err := t.st.Write(p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// pump reads the stream until it ends: data becomes input, a window the
// latest window change. A close from the browser ends the session as
// closed, a broken stream with causeLost.
func (t *webTerminal) pump(cancel context.CancelCauseFunc) {
	defer close(t.resize)
	for {
		data, cols, rows, err := t.st.Read()
		switch {
		case err != nil:
			_ = t.inW.Close() // the connector reads EOF
			if _, closed := errors.AsType[streamCloseError](err); closed {
				cancel(nil)
			} else {
				cancel(causeLost)
			}
			return
		case cols > 0:
			select { // only the latest window waits for the connector
			case <-t.resize:
			default:
			}
			t.resize <- Window{Cols: cols, Rows: rows}
		default:
			// ponytail: blocks while the connector reads no input, so a browser close
			// is seen only once it reads again, at idle timeout or max duration; read
			// into a bounded queue that ends the session on overflow if that matters.
			if _, err := t.inW.Write(data); err != nil {
				return // the session is over
			}
		}
	}
}
