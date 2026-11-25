package mcpserver

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// approvalNotice is what the gateway writes to stderr before it waits for
// an approval: the id, then the link on the next line.
var approvalNotice = regexp.MustCompile(`^Waiting for approval (\S+), timeout \S+\r\n(\S*)\r\n`)

// approvedLine follows the notice once the approval comes: the command runs.
var approvedLine = []byte("Approved\r\n")

// capture keeps stdout and stderr of one exec within MaxOutput together, in
// the order they arrive; the rest is read and dropped.
type capture struct {
	resource       string // the called resource: the gateway names it in its refusals
	mu             sync.Mutex
	left           int
	stdout, stderr bytes.Buffer
	truncated      bool
	noticeEnd      int  // length of stderr through the approval notice and approvedLine; 0: none
	approved       bool // approvedLine came: the call no longer waits for a decision
	approvalID     string
	approvalURL    string
	notice         chan struct{} // closed when the approval notice arrives
}

func newCapture(resource string) *capture {
	return &capture{resource: resource, left: MaxOutput, notice: make(chan struct{})}
}

// sink is one stream of a capture.
type sink struct {
	c   *capture
	buf *bytes.Buffer
}

func (w sink) Write(p []byte) (int, error) {
	c := w.c
	c.mu.Lock()
	defer c.mu.Unlock()
	n := min(len(p), c.left)
	w.buf.Write(p[:n])
	c.left -= n
	c.truncated = c.truncated || n < len(p)
	if w.buf == &c.stderr && !c.approved {
		c.scanNotice()
	}
	return len(p), nil
}

// scanNotice finds the approval notice at the start of stderr, then the
// approved line right after it.
func (c *capture) scanNotice() {
	b := c.stderr.Bytes()
	if c.noticeEnd == 0 {
		m := approvalNotice.FindSubmatchIndex(b)
		if m == nil {
			return
		}
		c.noticeEnd, c.approvalID, c.approvalURL = m[1], string(b[m[2]:m[3]]), string(b[m[4]:m[5]])
		close(c.notice)
	}
	if bytes.HasPrefix(b[c.noticeEnd:], approvedLine) {
		c.approved, c.noticeEnd = true, c.noticeEnd+len(approvedLine)
	}
}

// awaiting reports whether the gateway still waits for the approval: the
// notice came and nothing after it, the approved line included.
func (c *capture) awaiting() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.noticeEnd > 0 && !c.approved && c.stdout.Len() == 0 && c.stderr.Len() == c.noticeEnd
}

// wait returns the outcome once done reports the end of the session; after
// pendingAfter of waiting for an approval, or at the deadline while it
// still waits, it answers pending_approval.
func (c *capture) wait(ctx context.Context, done <-chan error, timeout time.Duration) ExecOutput {
	notice := c.notice
	var pending <-chan time.Time
	for {
		select {
		case err := <-done:
			return c.result(err)
		case <-notice:
			notice, pending = nil, time.After(pendingAfter)
		case <-pending:
			if c.awaiting() {
				return c.pendingOutput()
			}
			pending = nil
		case <-ctx.Done():
			if c.awaiting() {
				return c.pendingOutput()
			}
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return timedOut(timeout)
			}
			return ExecOutput{Status: "error", Message: "canceled"}
		}
	}
}

func (c *capture) pendingOutput() ExecOutput {
	c.mu.Lock()
	defer c.mu.Unlock()
	return ExecOutput{Status: "pending_approval", ApprovalID: c.approvalID, ApprovalURL: c.approvalURL,
		Message: "waiting for an approval; call ssh_exec again with the same resource and command once it is approved"}
}

// result is the outcome of a session that ended with err from Wait.
func (c *capture) result(err error) ExecOutput {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := ExecOutput{Stdout: c.stdout.String(), Stderr: c.stderr.String(), Truncated: c.truncated,
		ApprovalID: c.approvalID, ApprovalURL: c.approvalURL}
	code, ok := exitCode(err)
	if !ok {
		out.Status, out.Message = "error", "the session ended without an exit status"
		return out
	}
	out.ExitCode = &code
	out.Status, out.Message = gatewayStatus(out.Stdout+out.Stderr[c.noticeEnd:], code, c.resource)
	return out
}

func exitCode(err error) (int, bool) {
	if err == nil {
		return 0, true
	}
	if e, ok := errors.AsType[*ssh.ExitError](err); ok {
		return e.ExitStatus(), true
	}
	return 0, false
}
