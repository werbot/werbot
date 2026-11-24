package connectcli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

// mfaPrompt is the gateway question of an MFA round (contract §3.6); the code
// alphabet has no vowels. Only http and https URLs match. OpenSSH 8.4+ hands
// it to SSH_ASKPASS behind "(user@host) ".
var mfaPrompt = regexp.MustCompile(`^(?:\(\S+\) )?Open (https?://\S+) \(code ([B-DF-HJ-NP-TV-XZ]{4}-[B-DF-HJ-NP-TV-XZ]{4})\)( or enter TOTP)?: $`)

// ParseMFAPrompt splits the MFA prompt s into the page url and the confirmation
// code; totp reports that a TOTP code may be typed instead of confirming in the
// browser. Any other text is not ok.
func ParseMFAPrompt(s string) (url, code string, totp, ok bool) {
	m := mfaPrompt.FindStringSubmatch(s)
	if m == nil {
		return "", "", false, false
	}
	return m[1], m[2], m[3] != "", true
}

// KeyboardInteractive answers the gateway's MFA prompt: it opens the page with
// open (an error is ignored, the prompt shows the URL), then waits for the
// browser (empty answer, passkey prompt) or reads a TOTP line from in (prompt
// to out). Any other question is shown on out and answered with a line from in
// (without echo on a terminal); it is never answered by itself. Calls are
// serialized, so parallel connections prompt one at a time.
func KeyboardInteractive(open func(string) error, in io.Reader, out io.Writer) gossh.KeyboardInteractiveChallenge {
	var mu sync.Mutex
	rd := bufio.NewReader(in)
	return func(_, _ string, questions []string, echos []bool) ([]string, error) {
		mu.Lock()
		defer mu.Unlock()
		answers := make([]string, len(questions))
		for i, q := range questions {
			var err error
			if answers[i], err = answer(q, echos[i], open, in, rd, out); err != nil {
				return nil, err
			}
		}
		return answers, nil
	}
}

func answer(q string, echo bool, open func(string) error, in io.Reader, rd *bufio.Reader, out io.Writer) (string, error) {
	url, code, totp, ok := ParseMFAPrompt(q)
	if !ok {
		_, _ = fmt.Fprint(out, q)
		return readAnswer(in, rd, out, echo)
	}
	if open != nil {
		_ = open(url)
	}
	if !totp {
		_, err := fmt.Fprintf(out, "Opened %s (code %s), waiting for confirmation\n", url, code)
		return "", err
	}
	_, _ = fmt.Fprint(out, q)
	s, err := readAnswer(in, rd, out, true)
	if err == io.EOF { // no input: confirm in the browser
		return "", nil
	}
	return s, err
}

// readAnswer reads one line: on a terminal without echo unless echo, else from
// rd. End of input without a line is io.EOF.
func readAnswer(in io.Reader, rd *bufio.Reader, out io.Writer, echo bool) (string, error) {
	if f, ok := in.(interface{ Fd() uintptr }); ok && !echo && term.IsTerminal(int(f.Fd())) {
		b, err := term.ReadPassword(int(f.Fd()))
		_, _ = fmt.Fprintln(out)
		return string(b), err
	}
	s, err := rd.ReadString('\n')
	if err == io.EOF && s != "" {
		err = nil
	}
	return strings.TrimRight(s, "\r\n"), err
}

// mfaAuth is the keyboard-interactive method of c: the MFA round of the gateway.
func (c Config) mfaAuth(out io.Writer) gossh.AuthMethod {
	in := c.Stdin
	if in == nil {
		in = os.Stdin
	}
	return gossh.KeyboardInteractive(KeyboardInteractive(c.OpenBrowser, in, out))
}
