package shellclass

import (
	"slices"
	"strconv"
	"strings"
)

// check classifies a command whose class needs custom logic.
type check func(c *Classifier, args []word, host string, depth int) set

// rule is one entry of the built-in table.
type rule struct {
	base  set             // classes when no listed option is present
	flags map[string]set  // option ("-r", "--force") → classes; any match replaces base
	old   bool            // the first argument is an option cluster without "-" (tar)
	subs  map[string]set  // subcommand ("get", "system prune") → classes; unknown → opaque
	pre   map[string]bool // options with a value before the subcommand
	check check
}

// keyed builds a key → classes map from "|"-separated keys per class.
func keyed(m map[Class]string) map[string]set {
	out := map[string]set{}
	for c, keys := range m {
		for _, k := range strings.Split(keys, "|") {
			out[k] |= of(c)
		}
	}
	return out
}

var (
	pkgSubs = keyed(map[Class]string{
		Write:       "install|upgrade|update|add",
		Destructive: "remove|purge|del|erase|autoremove",
	})
	tarFlags = keyed(map[Class]string{
		Write: "-c|-x|-r|-u|-A|--create|--extract|--get|--append|--update|--concatenate|--catenate|--delete",
		Read:  "-t|-d|--list|--diff|--compare",
		Opaque: "-I|-F|--to-command|--use-compress-program|--checkpoint-action|--info-script|" +
			"--new-volume-script|--rsh-command|--rmt-command",
	})
)

// rules is the built-in table; init breaks the cycle rules → find → call → rules.
var rules map[string]rule

func init() { rules = buildRules() }

func buildRules() map[string]rule {
	m := map[string]rule{}
	add := func(r rule, names string) {
		for _, n := range strings.Fields(names) {
			m[n] = r
		}
	}
	add(rule{base: of(Read)}, "ls cat head tail less more grep egrep fgrep zgrep zcat wc stat file du df free "+
		"uptime uname hostname id whoami w who ps top pgrep pidof date echo pwd which type cd test [ true false : "+
		"sleep sort uniq cut tr awk jq diff cmp md5sum sha256sum lsblk lsof ss netstat dmesg journalctl git-upload-pack")
	add(rule{base: of(Read), flags: keyed(map[Class]string{Write: "-i|--in-place"})}, "sed")
	add(rule{base: of(Read), flags: keyed(map[Class]string{Opaque: "-d|-D|--decode"})}, "base64")
	add(rule{base: of(Read), flags: keyed(map[Class]string{Opaque: "-v"})}, "printf")
	add(rule{base: of(Write)}, "cp mv mkdir rmdir touch ln install tee chgrp kill pkill killall vi vim nano "+
		"unzip git-receive-pack")
	add(rule{base: of(Write), flags: keyed(map[Class]string{
		Destructive: "-r|-R|-f|--recursive|--force|--no-preserve-root"})}, "rm")
	add(rule{base: of(Write), flags: keyed(map[Class]string{Read: "-l", Destructive: "-r"})}, "crontab")
	add(rule{base: opaque, old: true, flags: tarFlags}, "tar")
	add(rule{base: of(Destructive)}, "dd truncate shred wipefs fdisk sfdisk parted reboot shutdown poweroff halt mkfs")
	add(rule{base: privileged}, "chown useradd usermod userdel groupadd passwd chpasswd visudo setcap")
	add(rule{base: of(Secrets)}, "printenv history")
	add(rule{check: setBuiltin}, "set")
	add(rule{check: find}, "find")
	add(rule{check: chmod}, "chmod")
	add(rule{check: shell}, "bash sh zsh dash ksh")
	add(rule{check: su}, "su")
	add(rule{subs: keyed(map[Class]string{
		Read:  "status|is-active|is-enabled|show|cat|list-units|list-timers",
		Write: "restart|start|stop|reload|enable|disable|daemon-reload|mask|unmask",
	})}, "systemctl")
	add(rule{pre: map[string]bool{"-n": true, "--namespace": true, "--context": true}, subs: keyed(map[Class]string{
		Read:        "get|describe|logs|top|explain|version|api-resources",
		Write:       "apply|create|patch|scale|rollout|edit|set|label|annotate",
		Destructive: "delete|drain",
	})}, "kubectl")
	add(rule{subs: keyed(map[Class]string{
		Read:        "ps|images|logs|inspect|stats",
		Write:       "start|stop|restart|pull",
		Destructive: "rm|rmi|prune|system prune|volume prune|image prune",
	})}, "docker")
	add(rule{pre: map[string]bool{"-C": true}, subs: keyed(map[Class]string{
		Read:  "status|log|diff|show",
		Write: "pull|fetch|checkout|commit|push",
	})}, "git")
	add(rule{subs: pkgSubs}, "apt apt-get yum dnf apk")
	addClients(add)
	return m
}

func (r rule) eval(c *Classifier, args []word, host string, depth int) set {
	switch {
	case r.check != nil:
		return r.check(c, args, host, depth)
	case r.subs != nil:
		return r.subcommand(args[1:])
	case r.flags == nil:
		return r.base
	case dynamicHead(args[1:]):
		return opaque
	}
	if s := r.flagClasses(args[1:]); s != 0 {
		return s
	}
	return r.base
}

func (r rule) flagClasses(args []word) set {
	var s set
	for i, a := range args {
		t := a.text
		switch {
		case t == "--":
			return s
		case i == 0 && r.old && !strings.HasPrefix(t, "-"):
			s |= r.cluster(t)
		case strings.HasPrefix(t, "--"):
			s |= r.long(t)
		case strings.HasPrefix(t, "-"):
			s |= r.cluster(t[1:])
		}
	}
	return s
}

func (r rule) cluster(letters string) set {
	var s set
	for i := range len(letters) {
		s |= r.flags["-"+letters[i:i+1]]
	}
	return s
}

// long matches "--name[=value]" and its unambiguous-or-not abbreviations.
func (r rule) long(t string) set {
	name, _, _ := strings.Cut(t, "=")
	var s set
	for k, v := range r.flags {
		if strings.HasPrefix(k, "--") && strings.HasPrefix(k, name) {
			s |= v
		}
	}
	return s
}

// subcommand classifies by the first operand, or the first two ("system prune").
func (r rule) subcommand(args []word) set {
	i := r.operand(args, 0)
	if i < 0 || dynamicHead(args[:i+1]) || !args[i].plain() {
		return opaque
	}
	if j := r.operand(args, i+1); j >= 0 && args[j].lit {
		if s, ok := r.subs[args[i].text+" "+args[j].text]; ok {
			return s
		}
	}
	if s, ok := r.subs[args[i].text]; ok {
		return s
	}
	return opaque
}

// operand returns the index of the first word from start that is neither an option nor a "pre" option value.
func (r rule) operand(args []word, start int) int {
	for i := start; i < len(args); i++ {
		t := args[i].text
		if !strings.HasPrefix(t, "-") || t == "-" {
			return i
		}
		if r.pre[t] {
			i++
		}
	}
	return -1
}

// setBuiltin: "set" alone prints every variable.
func setBuiltin(_ *Classifier, args []word, _ string, _ int) set {
	if len(args) == 1 {
		return of(Secrets)
	}
	return opaque
}

// chmodOpts are chmod options; the first other word is the mode.
var chmodOpts = []string{"-R", "-c", "-f", "-v", "--recursive", "--changes", "--silent", "--quiet", "--verbose",
	"--preserve-root", "--no-preserve-root"}

// chmod is write, or privilege for setuid/setgid and world-writable-all modes.
func chmod(_ *Classifier, args []word, _ string, _ int) set {
	args = args[1:]
	if dynamicHead(args) {
		return opaque
	}
	for _, a := range args {
		if slices.Contains(chmodOpts, a.text) || strings.HasPrefix(a.text, "--reference") {
			continue
		}
		if privilegedMode(a.text) {
			return privileged
		}
		break
	}
	return of(Write)
}

func privilegedMode(m string) bool {
	if n, err := strconv.ParseUint(m, 8, 32); err == nil {
		return n&0o6000 != 0 || n&0o777 == 0o777
	}
	for clause := range strings.SplitSeq(m, ",") {
		i := strings.IndexAny(clause, "+=")
		if i >= 0 && privilegedClause(clause[:i], clause[i+1:]) {
			return true
		}
	}
	return false
}

// privilegedClause: a set-id bit, or read-write-execute for everyone.
func privilegedClause(who, perms string) bool {
	if strings.Contains(perms, "s") {
		return true
	}
	everyone := who == "" || strings.Contains(who, "a") || containsAll(who, "ugo")
	return everyone && containsAll(perms, "rwx")
}

func containsAll(s, chars string) bool {
	for i := range len(chars) {
		if !strings.Contains(s, chars[i:i+1]) {
			return false
		}
	}
	return true
}
