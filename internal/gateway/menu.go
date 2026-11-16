package gateway

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"golang.org/x/term"
)

const menuPromptText = "Select resource or press Enter to exit > "

// menu lists the resources u may connect to and reads a number. It returns the
// chosen resource name, or "" and the exit code when the session ends here.
func (h *sshHandler) menu(s ssh.Session, u User) (string, int) {
	pol := h.p.ctl.Policy()
	if pol == nil || !h.p.ctl.Connected() {
		say(s, openText(h.p.reject(ErrNotReady), "", "", nil))
		return "", 1
	}
	rs := pol.Permitted(u, clientIP(s.RemoteAddr()), time.Now())
	if len(rs) == 0 {
		say(s, "You don't have access to any resources")
		return "", 1
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Hello %s, you have access to:", u.Login)
	for i, r := range rs {
		fmt.Fprintf(&b, "\n  %d  %s", i+1, r.Name)
	}
	say(s, b.String())
	line, err := term.NewTerminal(byteReader{s}, menuPromptText).ReadLine()
	line = strings.TrimSpace(line)
	if err != nil || line == "" { // Enter, or Ctrl-D
		say(s, "Bye")
		return "", 0
	}
	n, err := strconv.Atoi(line)
	if err != nil || n < 1 || n > len(rs) {
		say(s, "Resource not found")
		return "", 1
	}
	return rs[n-1].Name, 0
}

// errMenuInputTooLong ends a menu whose input filled term.Terminal's buffer.
var errMenuInputTooLong = errors.New("menu input too long")

// byteReader reads one byte at a time, so term.Terminal never buffers bytes
// past the selected line: the input typed after it reaches the connector. A
// client that ends the line with CRLF passes the LF to the target. An escape
// sequence that never completes fills the buffer, and term then reads into an
// empty slice; an SSH channel returns 0, nil for it, so ReadLine would spin.
type byteReader struct{ io.ReadWriter }

func (r byteReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, errMenuInputTooLong
	}
	return r.ReadWriter.Read(p[:1])
}
