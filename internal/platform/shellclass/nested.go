package shellclass

import (
	"slices"
	"strings"
)

// Commands that run other commands: find -exec, shells with -c, su -c.

var (
	findFlags = keyed(map[Class]string{Destructive: "-delete", Write: "-fprint|-fprint0|-fprintf|-fls"})
	findExec  = []string{"-exec", "-execdir", "-ok", "-okdir"}
)

// find is read, or the classes of its action options, plus the classes of
// every command it executes.
func find(c *Classifier, args []word, host string, depth int) set {
	args = args[1:]
	if dynamicHead(args) {
		return opaque
	}
	var flags, runs set
	for i := 0; i < len(args); i++ {
		flags |= findFlags[args[i].text]
		if !slices.Contains(findExec, args[i].text) {
			continue
		}
		end := execEnd(args, i+1)
		if end == i+1 {
			return opaque
		}
		runs |= c.call(args[i+1:end], host, depth+1)
		i = end
	}
	if flags == 0 {
		flags = of(Read)
	}
	return flags | runs
}

// execEnd returns the index of the ";" or "+" that ends a -exec command.
func execEnd(args []word, start int) int {
	for i := start; i < len(args); i++ {
		if args[i].lit && (args[i].text == ";" || args[i].text == "+") {
			return i
		}
	}
	return len(args)
}

// shellFlags are the short shell options allowed before the command string.
const shellFlags = "abcefhiklmnptuvxBCEHPT"

var shellLongs = []string{"--login", "--noprofile", "--norc", "--posix", "--noediting", "--restricted", "--verbose"}

// shell classifies "sh -c '<literal>'" by its command string; any other form is opaque.
func shell(c *Classifier, args []word, host string, depth int) set {
	cmd, ok := shellCommand(args[1:])
	if !ok {
		return opaque
	}
	return shellString(c, cmd, host, depth)
}

// shellCommand returns the command string that follows -c.
func shellCommand(args []word) (word, bool) {
	dashC := false
	for i := 0; i < len(args); i++ {
		if !args[i].lit {
			return word{}, false
		}
		switch shellArg(args[i].text) {
		case argBad:
			return word{}, false
		case argValue:
			i++
		case argCommand:
			dashC = true
		case argOperand:
			return args[i], dashC
		}
	}
	return word{}, false
}

type argKind int

const (
	argFlag argKind = iota
	argBad
	argValue   // an option whose value is the next word
	argCommand // a cluster with -c
	argOperand
)

var shellValueOpts = []string{"-o", "+o", "-O", "+O"}

func shellArg(t string) argKind {
	switch {
	case slices.Contains(shellValueOpts, t):
		return argValue
	case strings.HasPrefix(t, "--"):
		if slices.Contains(shellLongs, t) {
			return argFlag
		}
		return argBad
	case len(t) < 2 || strings.IndexByte("-+", t[0]) < 0:
		return argOperand
	case strings.Trim(t[1:], shellFlags) != "":
		return argBad
	case strings.Contains(t, "c"):
		return argCommand
	}
	return argFlag
}

var suSpec = optSpec{short: "lmpfP", value: "cgGsw", long: map[string]bool{"command": true,
	"session-command": true, "login": false, "shell": true, "group": true, "supp-group": true,
	"preserve-environment": false, "pty": false, "fast": false, "whitelist-environment": true}}

// su is privilege plus the classes of its -c command string; extra operands go to the shell.
func su(c *Classifier, args []word, host string, depth int) set {
	opts, operands, ok := scanOpts(args[1:], suSpec, false)
	if !ok {
		return privileged | opaque
	}
	users := 0
	for _, o := range operands {
		if !o.lit {
			return privileged | opaque
		}
		if o.text != "-" {
			users++
		}
	}
	if users > 1 {
		return privileged | opaque
	}
	for _, o := range opts {
		if o.name == "-c" || o.name == "--command" || o.name == "--session-command" {
			return privileged | shellString(c, o.val, host, depth)
		}
	}
	return privileged
}

func shellString(c *Classifier, cmd word, host string, depth int) set {
	if !cmd.lit {
		return opaque
	}
	return c.script(cmd.text, host, depth+1)
}
