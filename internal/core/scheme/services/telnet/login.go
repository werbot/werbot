package telnet

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"time"
)

// Login errors returned to the connector.
var (
	ErrLoginFailed  = errors.New("telnet login failed")
	ErrLoginTimeout = errors.New("telnet login timeout")
)

var (
	loginPromptRe    = regexp.MustCompile(`(?i)(login|username)\s*:\s*$`)
	passwordPromptRe = regexp.MustCompile(`(?i)password\s*:\s*$`)
	loginFailedRe    = regexp.MustCompile(`(?i)(login incorrect|authentication failed|access denied)`)
	shellPromptRe    = regexp.MustCompile(`[$#>%]\s*$`)
)

const (
	loginTailLimit  = 4096
	loginQuietPause = time.Second
)

type promptBuffer struct {
	tail []byte
}

func (b *promptBuffer) append(data []byte) {
	b.tail = append(b.tail, data...)
	if len(b.tail) > loginTailLimit {
		b.tail = b.tail[len(b.tail)-loginTailLimit:]
	}
}

// match consumes the buffer through the end of the prompt and returns any
// bytes that arrived after it.
func (b *promptBuffer) match(re *regexp.Regexp) ([]byte, bool) {
	loc := re.FindIndex(b.tail)
	if loc == nil {
		return nil, false
	}

	var leftover []byte
	if loc[1] < len(b.tail) {
		leftover = append([]byte(nil), b.tail[loc[1]:]...)
	}
	b.tail = b.tail[:0]
	return leftover, true
}

// Login authenticates against a telnet target using its login/password
// prompts. It returns only the output produced after the password prompt;
// the target password is stripped from that output.
func Login(ctx context.Context, c *Conn, login, password string) ([]byte, error) {
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		deadline = time.Now().Add(10 * time.Second)
	}

	prompt := &promptBuffer{}
	leftover, err := readPrompt(ctx, c, prompt, loginPromptRe, deadline)
	if err != nil {
		return nil, err
	}

	if err := writeLoginLine(c, login); err != nil {
		return nil, err
	}

	leftover, err = readPromptWith(ctx, c, prompt, passwordPromptRe, deadline, leftover)
	if err != nil {
		return nil, err
	}

	if err := writeLoginLine(c, password); err != nil {
		return nil, err
	}

	out, err := readAfterPassword(ctx, c, deadline, leftover)
	if password != "" {
		out = bytes.ReplaceAll(out, []byte(password), nil)
	}
	if err != nil {
		return out, err
	}

	_ = c.SetDeadline(time.Time{})
	return out, nil
}

func readPrompt(ctx context.Context, c *Conn, prompt *promptBuffer, re *regexp.Regexp, deadline time.Time) ([]byte, error) {
	return readPromptWith(ctx, c, prompt, re, deadline, nil)
}

func readPromptWith(ctx context.Context, c *Conn, prompt *promptBuffer, re *regexp.Regexp, deadline time.Time, initial []byte) ([]byte, error) {
	prompt.append(initial)
	if leftover, ok := prompt.match(re); ok {
		return leftover, nil
	}

	buf := make([]byte, 4096)
	for {
		if ctx.Err() != nil {
			return nil, ErrLoginTimeout
		}
		_ = c.SetDeadline(deadline)

		n, err := c.Read(buf)
		if n > 0 {
			prompt.append(buf[:n])
			if leftover, ok := prompt.match(re); ok {
				return leftover, nil
			}
		}

		if err != nil {
			if ctx.Err() != nil {
				return nil, ErrLoginTimeout
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				return nil, ErrLoginTimeout
			}
			return nil, err
		}
	}
}

func readAfterPassword(ctx context.Context, c *Conn, deadline time.Time, initial []byte) ([]byte, error) {
	out := append([]byte(nil), initial...)
	prompt := &promptBuffer{}
	prompt.append(initial)

	if done, err := loginResult(prompt.tail); done {
		return out, err
	}

	lastData := time.Now()
	buf := make([]byte, 4096)

	for {
		if ctx.Err() != nil {
			return out, ErrLoginTimeout
		}

		readDeadline := lastData.Add(loginQuietPause)
		if deadline.Before(readDeadline) {
			readDeadline = deadline
		}
		_ = c.SetDeadline(readDeadline)

		n, err := c.Read(buf)
		if n > 0 {
			out = append(out, buf[:n]...)
			prompt.append(buf[:n])
			lastData = time.Now()

			if done, resultErr := loginResult(prompt.tail); done {
				return out, resultErr
			}
		}

		if err == nil {
			continue
		}

		if ctx.Err() != nil {
			return out, ErrLoginTimeout
		}

		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			if !time.Now().Before(deadline) {
				return out, ErrLoginTimeout
			}
			if time.Since(lastData) >= loginQuietPause {
				return out, nil
			}
			continue
		}

		if errors.Is(err, io.EOF) {
			return out, err
		}
		return out, err
	}
}

func loginResult(tail []byte) (bool, error) {
	switch {
	case loginFailedRe.Match(tail):
		return true, ErrLoginFailed
	case loginPromptRe.Match(tail):
		return true, ErrLoginFailed
	case shellPromptRe.Match(tail):
		return true, nil
	default:
		return false, nil
	}
}

func writeLoginLine(c *Conn, value string) error {
	if _, err := c.Write([]byte(value + "\r\n")); err != nil {
		return fmt.Errorf("telnet login write: %w", err)
	}
	return nil
}
