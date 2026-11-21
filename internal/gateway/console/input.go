package console

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"sync"

	"github.com/werbot/werbot/internal/gateway"
)

const keyCtrlC = 0x03

// errInputOverflow is an escape sequence that filled the line editor's
// buffer: term.Terminal then reads into an empty slice.
var errInputOverflow = errors.New("console: input escape sequence too long")

// input is the user's terminal input. The pump records every byte; while a
// prompt waits for a line (armed) it passes the input, without bracketed-paste
// markers, to the line editor that reads it through Read; at any other time it
// drops it, and Ctrl-C calls the interrupt handler of the running check or line.
type input struct {
	rec *gateway.LiveSink

	mu        sync.Mutex
	more      *sync.Cond // buf grew or the input closed
	armed     bool
	closed    bool
	buf       []byte
	tail      []byte // the start of a paste marker split across reads
	interrupt func() // called with mu held: it must not block
	ctrlC     bool   // the last Read ended at Ctrl-C; read by the line editor's goroutine only
}

func newInput(src io.Reader, rec *gateway.LiveSink) *input {
	in := &input{rec: rec}
	in.more = sync.NewCond(&in.mu)
	go in.pump(src)
	return in
}

// pump reads src until it fails; the front closes it when the session ends.
func (in *input) pump(src io.Reader) {
	b := make([]byte, 32<<10)
	for {
		n, err := src.Read(b)
		if n > 0 {
			in.rec.Input(b[:n])
			in.feed(b[:n])
		}
		if err != nil {
			in.close()
			return
		}
	}
}

func (in *input) feed(p []byte) {
	in.mu.Lock()
	defer in.mu.Unlock()
	if !in.armed {
		for range bytes.Count(p, []byte{keyCtrlC}) {
			if in.interrupt != nil {
				in.interrupt()
			}
		}
		return
	}
	in.buf = append(in.buf, in.stripPaste(p)...)
	in.more.Broadcast()
}

var pasteMarkers = [][]byte{[]byte("\x1b[200~"), []byte("\x1b[201~")}

// stripPaste removes the bracketed-paste markers from p, keeping a trailing
// start of one for the next read: in paste mode term.Terminal would take
// control characters and lines of any length (Ruling G28).
func (in *input) stripPaste(p []byte) []byte {
	b := slices.Concat(in.tail, p)
	in.tail = nil
	for n := -1; n != len(b); { // removing one marker can join another
		n = len(b)
		for _, m := range pasteMarkers {
			b = bytes.ReplaceAll(b, m, nil)
		}
	}
	for i := max(0, len(b)-len(pasteMarkers[0])+1); i < len(b); i++ {
		if bytes.HasPrefix(pasteMarkers[0], b[i:]) || bytes.HasPrefix(pasteMarkers[1], b[i:]) {
			in.tail = bytes.Clone(b[i:])
			return b[:i]
		}
	}
	return b
}

// Read gives the line editor the input up to and including the first byte
// that ends or interrupts a line, so the editor never holds input typed after
// it. An empty p fails (Ruling G29); a closed input is io.EOF.
func (in *input) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, errInputOverflow
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	for len(in.buf) == 0 && !in.closed {
		in.more.Wait()
	}
	if len(in.buf) == 0 {
		return 0, io.EOF
	}
	n := len(in.buf)
	if i := bytes.IndexAny(in.buf, "\r\n\x03"); i >= 0 {
		n = i + 1
		if in.buf[i] == keyCtrlC && i > 0 { // Ctrl-C alone: term drops the echo of keys read with it
			n = i
		}
	}
	n = copy(p, in.buf[:n])
	in.ctrlC = in.buf[n-1] == keyCtrlC
	in.buf = in.buf[n:]
	return n, nil
}

// arm passes the input typed from now on to the line editor.
func (in *input) arm() {
	in.mu.Lock()
	in.armed, in.buf, in.tail, in.ctrlC = true, nil, nil, false
	in.mu.Unlock()
}

// disarm drops the input until the next arm; Ctrl-C calls interrupt.
func (in *input) disarm(interrupt func()) {
	in.mu.Lock()
	in.armed, in.interrupt = false, interrupt
	in.mu.Unlock()
}

// close ends the input: Read returns io.EOF once buf is empty.
func (in *input) close() {
	in.mu.Lock()
	in.closed = true
	in.more.Broadcast()
	in.mu.Unlock()
}
