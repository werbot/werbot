package console

import (
	"bytes"
	"context"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxDirBytes = 1024
	cdSyntax    = "cd: use a plain path without quotes or expansions"
)

// cd changes the cwd to its argument (home without one): the target, then the
// directory the target resolves it to, must pass the check. Each check is of
// the script that runs on the target, whose pwd reads the folder, so folder
// rules see it. A refusal, a failed cd or an answer that is not a clean
// directory keeps the cwd.
func (c *console) cd(ctx context.Context, args []string) {
	target, ok := c.cdTarget(args)
	if !ok {
		c.fail(cdSyntax)
		return
	}
	chk, ok := c.check(ctx, cdScript(target))
	if !ok {
		return
	}
	var out answer
	if c.last = c.runOnTarget(ctx, cdScript(target), &out); c.last != 0 {
		return
	}
	dir, ok := out.dir()
	if !ok {
		c.fail("cd: cannot use this directory")
		return
	}
	if dir != target {
		if chk, ok = c.check(ctx, cdScript(dir)); !ok {
			return
		}
	}
	c.cwd = dir
	c.s.Recording.Marker(marker("cd -- "+quote(dir), chk.Classes))
}

// cdScript changes to dir and prints the folder it resolves to.
func cdScript(dir string) string {
	return "cd -- " + quote(dir) + " && pwd -P"
}

// cdTarget is the clean absolute directory of a cd with zero or one plain
// path argument (Ruling G32).
func (c *console) cdTarget(args []string) (string, bool) {
	switch {
	case len(args) == 0:
		return c.home, true
	case len(args) > 1 || !plainArg(args[0]):
		return "", false
	}
	dir := args[0]
	if rest, ok := strings.CutPrefix(dir, "~"); ok {
		dir = c.home + "/" + rest
	} else if !path.IsAbs(dir) {
		dir = c.cwd + "/" + dir
	}
	return path.Clean(dir), true
}

// plainArg is a cd argument of letters, digits and "/._-+,:@%=", not starting
// with "-", after an optional leading "~" alone or "~/".
func plainArg(arg string) bool {
	rest, tilde := strings.CutPrefix(arg, "~")
	return !strings.HasPrefix(arg, "-") && plainPath(rest) && (!tilde || rest == "" || rest[0] == '/')
}

func plainPath(s string) bool {
	return !strings.ContainsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("/._-+,:@%=", r)
	})
}

// answer keeps the first bytes of a target answer: enough for one directory.
type answer struct{ bytes.Buffer }

func (a *answer) Write(p []byte) (int, error) {
	if room := maxDirBytes + 2 - a.Len(); room > 0 {
		a.Buffer.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}

// dir is the answer as a directory: one line, absolute, clean, valid UTF-8,
// without control characters, at most 1024 bytes (Ruling G31).
func (a *answer) dir() (string, bool) {
	d, ok := strings.CutSuffix(a.String(), "\n")
	return d, ok && len(d) <= maxDirBytes && utf8.ValidString(d) && path.IsAbs(d) && path.Clean(d) == d &&
		!strings.ContainsFunc(d, unicode.IsControl)
}

// quote is s as one single-quoted shell word.
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
