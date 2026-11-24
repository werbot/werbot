package cli

import (
	"fmt"
	"io"

	"github.com/werbot/werbot/internal/connectcli"
)

// IsAskpass reports that werbot was started as the SSH_ASKPASS helper of
// werbot ssh: args (the command line without the program name) is exactly the
// prompt, ssh was told to always use the helper and the helper is self.
func IsAskpass(args []string, getenv func(string) string, self string) bool {
	return len(args) == 1 && getenv("SSH_ASKPASS_REQUIRE") == "force" && getenv("SSH_ASKPASS") == self
}

// stdio is the fallback terminal: stdin and stderr of the helper, which ssh
// leaves connected to the user's own streams.
type stdio struct {
	io.Reader
	io.Writer
}

// RunAskpass answers prompt on the controlling terminal (stdin and stderr when
// there is none) and writes the answer to stdout for ssh.
func RunAskpass(env Env, prompt string) error {
	tty, err := openTTY()
	if err != nil {
		return Askpass(env, stdio{env.Stdin, env.Stderr}, prompt)
	}
	defer func() { _ = tty.Close() }()
	return Askpass(env, tty, prompt)
}

// Askpass handles one ssh prompt: the MFA prompt of the gateway opens the page
// and waits or reads a TOTP, any other prompt is shown and read from tty as it
// is. The answer goes to env.Stdout, one line.
func Askpass(env Env, tty io.ReadWriter, prompt string) error {
	answers, err := connectcli.KeyboardInteractive(env.OpenBrowser, tty, tty)("", "", []string{prompt}, []bool{false})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(env.Stdout, answers[0])
	return err
}
