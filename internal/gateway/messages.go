package gateway

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"strings"
)

const usageText = "Use ssh <login>+<resource>@<host> to run commands or forward ports"

// Errors of an MFA step; the user reads their text.
var (
	ErrMFATimeout         = errors.New("Permission denied (MFA timeout)")        //nolint:staticcheck // the user reads this text
	ErrMFATooManyAttempts = errors.New("Too many MFA attempts, try again later") //nolint:staticcheck // the user reads this text
	// ErrMFARequired is an mfa obligation the front cannot ask for: a web
	// terminal closes "mfa_required", a forwarded port is refused.
	ErrMFARequired = errors.New("MFA required, reconnect to confirm")
)

// mfaText is the user text of a failed MFA step; "" for any other error.
func mfaText(err error) string {
	for _, e := range []error{ErrMFATimeout, ErrMFATooManyAttempts, ErrMFARequired} {
		if errors.Is(err, e) {
			return e.Error()
		}
	}
	return ""
}

// promptText asks for the second factor: the confirmation page and its code,
// or a TOTP code when one meets the rule.
func promptText(url, code string, totp bool) string {
	if totp {
		return fmt.Sprintf("Open %s (code %s) or enter TOTP: ", url, code)
	}
	return fmt.Sprintf("Open %s (code %s): ", url, code)
}

// openText is the user text for an Open error; denied and unknown resources read the same.
func openText(err error, resource string, kind Kind, pol *Policy) string {
	switch {
	case errors.Is(err, ErrBusy), errors.Is(err, ErrSpoolMax):
		return "Gateway is busy, try again later"
	case errors.Is(err, ErrUnsupportedType):
		r, _ := pol.Resource(resource)
		return fmt.Sprintf("Resource type %q is not supported by this gateway", r.Type)
	case errors.Is(err, ErrUnsupportedKind):
		return fmt.Sprintf("Resource %s does not support %s sessions", resource, kind)
	case errors.Is(err, ErrShellDenied):
		return "Interactive shell is not allowed on " + resource
	}
	return cmp.Or(agentRunText(err), mfaText(err), CommandErrorText(err))
}

// runText is the user text for a connector error.
func runText(err error, resource string) string {
	if errors.Is(err, ErrLoginFailed) {
		return "Login to " + resource + " failed"
	}
	if errors.Is(err, ErrHostKeyMismatch) {
		return "SSH host key mismatch for " + resource
	}
	if errors.Is(err, ErrConsoleUnavailable) {
		return ErrConsoleUnavailable.Error()
	}
	return "Connection to " + resource + " failed"
}

// say writes msg as one line with CRLF line ends, as a terminal needs them.
func say(w io.Writer, msg string) {
	_, _ = io.WriteString(w, strings.ReplaceAll(msg, "\n", "\r\n")+"\r\n")
}

// closeText is what a terminal reads when the sweeper or a terminate ends it.
var closeText = map[string]string{
	string(causeIdle): "Session closed: idle timeout",
	string(causeMax):  "Session closed: max duration reached",
	"terminated":      "Session closed: terminated",
}
