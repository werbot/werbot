package shellclass

import (
	"strings"
)

// call classifies one simple command; wrappers are stripped one per depth level.
func (c *Classifier) call(args []word, host string, depth int) set {
	out := argClasses(args)
	for ; depth <= maxDepth; depth++ {
		rest, s, done := c.step(args, host, depth)
		out |= s
		if done {
			return out
		}
		args = rest
	}
	return out | of(Opaque)
}

// step classifies args, or strips a wrapper and returns the wrapped command (done == false).
func (c *Classifier) step(args []word, host string, depth int) ([]word, set, bool) {
	if looseOption(args) {
		return nil, of(Opaque), true
	}
	name := normalizeName(args[0].text)
	if s, ok := c.match(name, args); ok {
		return nil, s, true
	}
	if w, ok := wrappers[name]; ok {
		return w.unwrap(args[1:])
	}
	return nil, c.command(name, args, host, depth), true
}

// looseOption reports an argv0, or an option word before "--", that is not
// fully literal or holds a glob or brace pattern (Ruling G3).
func looseOption(args []word) bool {
	if !args[0].plain() {
		return true
	}
	for _, a := range args[1:] {
		if a.text == "--" {
			return false
		}
		if strings.HasPrefix(a.text, "-") && !a.plain() {
			return true
		}
	}
	return false
}

// dynamicHead reports a word before "--" that starts with an expansion and so
// may turn into an option.
func dynamicHead(args []word) bool {
	for _, a := range args {
		if a.text == "--" {
			return false
		}
		if strings.HasPrefix(a.text, marker) {
			return true
		}
	}
	return false
}

// command classifies a command that is not a wrapper.
func (c *Classifier) command(name string, args []word, host string, depth int) set {
	r, ok := rules[name]
	if !ok && strings.HasPrefix(name, "mkfs.") {
		r, ok = rules["mkfs"]
	}
	if !ok {
		return of(Opaque)
	}
	s := r.eval(c, args, host, depth)
	if s&of(Write, Destructive) != 0 {
		for _, a := range args[1:] {
			s |= sudoers(a.text)
		}
	}
	return s
}

// wrapper describes a command that runs another command given after its own options.
type wrapper struct {
	spec   optSpec
	assign bool   // NAME=VALUE words may follow the options
	arg    bool   // one argument precedes the command (a duration)
	adds   set    // classes of the wrapper itself
	alone  set    // classes when no command follows
	query  string // short options that only report on the command
}

var (
	privileged = of(Privilege)
	opaque     = of(Opaque)
	wrappers   = map[string]wrapper{
		"env": {spec: optSpec{short: "i0", value: "uC", long: map[string]bool{
			"ignore-environment": false, "null": false, "unset": true, "chdir": true}}, assign: true, alone: of(Secrets)},
		"command": {spec: optSpec{short: "pvV"}, query: "vV", alone: opaque},
		"busybox": {alone: opaque},
		"exec":    {spec: optSpec{short: "cl", value: "a"}, alone: opaque},
		"nohup":   {alone: opaque},
		"nice":    {spec: optSpec{short: "0123456789", value: "n", long: map[string]bool{"adjustment": true}}, alone: opaque},
		"time":    {spec: optSpec{short: "pv", long: map[string]bool{"portability": false, "verbose": false}}, alone: opaque},
		"timeout": {spec: optSpec{short: "v", value: "sk", long: map[string]bool{"signal": true, "kill-after": true,
			"preserve-status": false, "foreground": false, "verbose": false}}, arg: true, alone: opaque},
		"stdbuf": {spec: optSpec{value: "ioe", long: map[string]bool{"input": true, "output": true, "error": true}}, alone: opaque},
		"xargs": {spec: optSpec{short: "0rtpx", value: "nLPsdEIa", long: map[string]bool{"null": false,
			"no-run-if-empty": false, "verbose": false, "interactive": false, "exit": false, "max-args": true,
			"max-lines": true, "max-procs": true, "max-chars": true, "delimiter": true, "eof": true,
			"replace": true, "arg-file": true}}, alone: of(Read)},
		"sudo": {spec: optSpec{short: "AbEHiKklnPSsVv", value: "CDgpRrTtUu", long: map[string]bool{"user": true,
			"group": true, "login": false, "shell": false, "preserve-env": false, "non-interactive": false,
			"set-home": false, "stdin": false, "background": false, "chdir": true, "prompt": true}},
			assign: true, adds: privileged, alone: privileged},
		"doas":   {spec: optSpec{short: "Lns", value: "Cu"}, adds: privileged, alone: privileged},
		"pkexec": {spec: optSpec{long: map[string]bool{"user": true, "disable-internal-agent": false, "keep-cwd": false}}, adds: privileged, alone: privileged},
	}
)

// unwrap strips the wrapper's options from args; done is true when no command follows or on an error.
func (w wrapper) unwrap(args []word) (rest []word, s set, done bool) {
	opts, rest, ok := scanOpts(args, w.spec, true)
	if !ok {
		return nil, w.adds | opaque, true
	}
	if w.queried(opts) {
		return nil, w.adds | of(Read), true
	}
	s = w.adds
	if w.assign {
		var a set
		rest, a = skipAssigns(rest)
		s |= a
	}
	if w.arg {
		if len(rest) == 0 || !rest[0].lit {
			return nil, s | opaque, true
		}
		rest = rest[1:]
	}
	if len(rest) == 0 {
		return nil, s | w.alone, true
	}
	return rest, s, false
}

func (w wrapper) queried(opts []opt) bool {
	for _, o := range opts {
		if len(o.name) == 2 && strings.Contains(w.query, o.name[1:]) {
			return true
		}
	}
	return false
}

// skipAssigns drops leading NAME=VALUE words; a hijack variable adds opaque.
func skipAssigns(args []word) ([]word, set) {
	var s set
	for len(args) > 0 {
		name, _, ok := strings.Cut(args[0].text, "=")
		if !ok || !validName(name) {
			break
		}
		s |= hijack(name)
		args = args[1:]
	}
	return args, s
}

func validName(s string) bool {
	if s == "" || (s[0] >= '0' && s[0] <= '9') {
		return false
	}
	return strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_") == ""
}
