package shellclass

import (
	"slices"
	"strings"
)

// operandMode says which operands of a command are written.
type operandMode uint8

const (
	opRead   operandMode = iota
	opWrite              // every operand
	opTarget             // the last of two or more operands (none with -t DIR)
	opMove               // every operand
	opInOut              // every operand after the first (uniq IN OUT)
)

// pathCheck extracts the paths of a command whose arguments need custom logic.
type pathCheck func(w *pathWalk, args []word, sc scope)

// pathRole is how a known command touches paths (Ruling G1). Options take no
// value unless spec says so; unknown options are flags.
type pathRole struct {
	spec    optSpec
	mode    operandMode
	expr    []string          // non-nil: the first operand is an expression unless one of these options is given
	writes  []string          // options that make every operand written
	recurse []string          // options that make operand refs subtrees
	tree    bool              // operand refs are always subtrees
	bare    bool              // without operands the command reads the cwd
	opts    map[string]Access // options whose value is a path
	into    []string          // options whose value is a folder the command writes into
	check   pathCheck
}

// targetOpts name the folder that cp, mv, install and ln write into.
var targetOpts = []string{"-t", "--target-directory"}

var (
	pathRoles map[string]pathRole
	cwdMovers = []string{"pushd", "popd"}
	// shellState are builtins that run text or change how later commands
	// and cd behave in this shell; their paths are unresolved.
	shellState = []string{"eval", "source", ".", "builtin", "enable", "alias", "shopt", "trap"}
)

// init breaks the cycle pathRoles → checks → call → pathRoles.
func init() { pathRoles = buildPathRoles() }

func buildPathRoles() map[string]pathRole {
	m := map[string]pathRole{}
	add := func(r pathRole, names string) {
		r.spec.loose = true
		for _, n := range strings.Fields(names) {
			m[n] = r
		}
	}
	read, write := map[string]Access{}, map[string]Access{}
	for _, n := range []string{"-f", "--file", "-X", "--exclude-from", "--files0-from", "--reference", "-r", "--random-source"} {
		read[n] = AccessRead
	}
	for _, n := range []string{"-o", "--output", "-T", "--temporary-directory"} {
		write[n] = AccessWrite
	}
	add(pathRole{}, "cat zcat md5sum sha256sum stat file wc base64 git-upload-pack")
	add(pathRole{spec: optSpec{value: "nc", long: longs("lines bytes")}}, "head")
	add(pathRole{spec: optSpec{value: "ncs", long: longs("lines bytes pid sleep-interval max-unchanged-stats")}}, "tail")
	add(pathRole{spec: optSpec{value: "bcdf", long: longs("bytes characters delimiter fields output-delimiter")}}, "cut")
	add(pathRole{spec: optSpec{value: "bhjkoOpPtTxyz", long: longs("log-file LOG-FILE pattern prompt tag")},
		opts: map[string]Access{"-o": AccessWrite, "-O": AccessWrite, "--log-file": AccessWrite, "--LOG-FILE": AccessWrite}}, "less")
	add(pathRole{spec: optSpec{value: "n"}}, "more")
	add(pathRole{spec: optSpec{value: "CDFILSUWXx"}, opts: read}, "diff")
	add(pathRole{spec: optSpec{value: "in"}}, "cmp")
	add(pathRole{spec: optSpec{value: "kotSTy", long: longs("key output field-separator buffer-size temporary-directory " +
		"files0-from random-source")},
		opts: map[string]Access{"-o": AccessWrite, "--output": AccessWrite, "-T": AccessWrite,
			"--temporary-directory": AccessWrite, "--files0-from": AccessRead, "--random-source": AccessRead}}, "sort")
	add(pathRole{spec: optSpec{value: "fsw", long: longs("skip-fields skip-chars check-chars")}, mode: opInOut}, "uniq")
	add(pathRole{spec: optSpec{value: "ABCDdefm", long: longs("regexp file max-count context after-context before-context " +
		"include exclude exclude-from exclude-dir label devices directories binary-files")},
		expr: []string{"-e", "-f", "--regexp", "--file"}, recurse: []string{"-r", "-R", "--recursive", "--dereference-recursive"},
		opts: read}, "grep egrep fgrep zgrep")
	add(pathRole{spec: optSpec{value: "efl", long: longs("expression file line-length")},
		expr: []string{"-e", "-f", "--expression", "--file"}, writes: []string{"-i", "--in-place"}, opts: read}, "sed")
	add(pathRole{spec: optSpec{value: "fvF", long: longs("file assign field-separator")},
		expr: []string{"-f", "--file"}, opts: read}, "awk")
	add(pathRole{spec: optSpec{value: "fL", long: longs("from-file indent")},
		expr: []string{"-f", "--from-file"}, opts: map[string]Access{"-f": AccessRead, "--from-file": AccessRead}}, "jq")
	add(pathRole{spec: optSpec{value: "ITw", long: longs("ignore hide format sort time-style width tabsize block-size quoting-style")},
		bare: true, recurse: []string{"-R", "--recursive"}}, "ls")
	add(pathRole{spec: optSpec{value: "dBtX", long: longs("max-depth block-size threshold exclude exclude-from files0-from time-style")},
		bare: true, tree: true, opts: read}, "du")
	add(pathRole{spec: optSpec{value: "tS", long: longs("target-directory suffix")}, mode: opTarget, into: targetOpts,
		recurse: []string{"-r", "-R", "-a", "--recursive", "--archive"}}, "cp")
	add(pathRole{spec: optSpec{value: "Sgmot", long: longs("target-directory suffix group mode owner")}, mode: opTarget,
		into: targetOpts, writes: []string{"-d", "--directory"}}, "install")
	add(pathRole{spec: optSpec{value: "tS", long: longs("target-directory suffix")}, mode: opTarget, into: targetOpts}, "ln")
	add(pathRole{spec: optSpec{value: "tS", long: longs("target-directory suffix")}, mode: opMove, into: targetOpts}, "mv")
	add(pathRole{mode: opWrite, recurse: []string{"-r", "-R", "--recursive"}}, "rm")
	add(pathRole{mode: opWrite}, "rmdir tee git-receive-pack nano")
	add(pathRole{spec: optSpec{value: "m", long: longs("mode context")}, mode: opWrite}, "mkdir")
	add(pathRole{spec: optSpec{value: "dtr", long: longs("date reference")}, mode: opWrite, opts: read}, "touch")
	add(pathRole{spec: optSpec{value: "sr", long: longs("size reference")}, mode: opWrite, opts: read}, "truncate")
	add(pathRole{spec: optSpec{value: "ns", long: longs("iterations size random-source")}, mode: opWrite, opts: read}, "shred")
	add(pathRole{spec: optSpec{value: "cSsTtUuWwi"}, mode: opWrite}, "vi vim")
	add(pathRole{spec: optSpec{long: longs("from reference")}, mode: opWrite, expr: []string{"--reference"},
		recurse: []string{"-R", "--recursive"}, opts: read}, "chown chgrp")
	add(pathRole{check: chmodPaths}, "chmod")
	add(pathRole{check: findPaths}, "find")
	add(pathRole{check: tarPaths}, "tar")
	add(pathRole{check: ddPaths}, "dd")
	add(pathRole{check: unzipPaths}, "unzip")
	add(pathRole{check: copyRole{spec: scpSpec, reads: []string{"-f"}, writes: []string{"-t"}, recurse: []string{"-r"}}.paths}, "scp")
	add(pathRole{check: copyRole{spec: rsyncSpec, reads: []string{"--sender"}, writes: []string{"--server"},
		recurse: []string{"-r", "-a", "--recursive", "--archive"}, opts: map[string]Access{"--log-file": AccessWrite}}.paths}, "rsync")
	add(pathRole{check: shellPaths}, "bash sh zsh dash ksh")
	add(pathRole{check: suPaths}, "su")
	return m
}

// hasOpt is has with long option abbreviations ("--rec" for "--recursive").
func hasOpt(opts []opt, names []string) bool {
	return slices.ContainsFunc(opts, func(o opt) bool { return slices.ContainsFunc(names, o.matches) })
}

func (o opt) matches(name string) bool {
	return o.name == name || (len(o.name) > 3 && strings.HasPrefix(o.name, "--") && strings.HasPrefix(name, o.name))
}

// scanRole is scanOpts with abbreviated long options spelled out first, so
// a value given as the next word is taken as getopt_long does.
func scanRole(args []word, spec optSpec) ([]opt, []word, bool) {
	return scanOpts(fullLongs(args, spec), spec, false)
}

// fullLongs replaces "--abbrev[=v]" before "--" with the one long option of
// spec it abbreviates; ambiguous or unknown names stay as they are.
func fullLongs(args []word, spec optSpec) []word {
	out := slices.Clone(args)
	for i, a := range out {
		if a.text == "--" {
			break
		}
		name, _, _ := strings.Cut(strings.TrimPrefix(a.text, "--"), "=")
		if full, ok := uniquePrefix(spec.long, name); ok && strings.HasPrefix(a.text, "--") {
			out[i].text = "--" + full + a.text[2+len(name):]
		}
	}
	return out
}

func uniquePrefix(long map[string]bool, name string) (string, bool) {
	if _, known := long[name]; known || name == "" {
		return "", false
	}
	full, n := "", 0
	for k := range long {
		if strings.HasPrefix(k, name) {
			full, n = k, n+1
		}
	}
	return full, n == 1
}

// optAccess returns the access of a path-valued option.
func optAccess(m map[string]Access, o opt) (Access, bool) {
	for name, a := range m {
		if o.matches(name) {
			return a, true
		}
	}
	return "", false
}

// eval adds the paths of a command described by the role's fields.
func (r pathRole) eval(w *pathWalk, args []word, sc scope) {
	opts, ops, ok := scanRole(args[1:], r.spec)
	if !ok {
		w.unresolved = true
		return
	}
	w.optPaths(opts, r.opts, sc)
	for _, o := range opts {
		if slices.ContainsFunc(r.into, o.matches) {
			w.path(o.val, AccessWrite, true, sc)
		}
	}
	tree := r.tree || hasOpt(opts, r.recurse)
	if r.expr != nil && len(ops) > 0 && !hasOpt(opts, r.expr) {
		ops = ops[1:]
	}
	r.operands(w, opts, ops, tree, sc)
}

func (r pathRole) operands(w *pathWalk, opts []opt, ops []word, tree bool, sc scope) {
	n := len(ops)
	dest := r.destination(opts, n)
	if n == 0 && (r.bare || tree) {
		w.cwdRef(r.access(opts, 0, 1), tree, sc)
	}
	for i, o := range ops {
		if o.text != "-" {
			w.path(o, r.access(opts, i, n), tree || i == dest, sc)
		}
	}
	if dest == n {
		w.cwdRef(AccessWrite, true, sc)
	}
}

// destination returns the index of the operand that may be a folder written
// into, so a subtree; -1 for none, n when a lone operand goes into the cwd.
func (r pathRole) destination(opts []opt, n int) int {
	switch {
	case r.mode != opTarget && r.mode != opMove, hasOpt(opts, r.into), hasOpt(opts, r.writes):
		return -1
	case n == 1:
		return 1
	}
	return n - 1
}

// access returns the access of operand i of n.
func (r pathRole) access(opts []opt, i, n int) Access {
	switch {
	case r.mode == opWrite, r.mode == opMove, hasOpt(opts, r.writes),
		r.mode == opTarget && n > 1 && i == n-1 && !hasOpt(opts, r.into),
		r.mode == opInOut && i > 0:
		return AccessWrite
	}
	return AccessRead
}
