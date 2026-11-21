package shellclass

import (
	"slices"
	"strings"
)

// Commands whose paths need custom logic.

// chmodOption reports a chmod option; any other word before the files is the mode.
func chmodOption(t string) bool {
	return slices.Contains(chmodOpts, t) || (len(t) > 1 && t[0] == '-' && strings.Trim(t[1:], "Rcfv") == "")
}

func chmodRecursive(a word) bool {
	return a.text == "--recursive" || (chmodOption(a.text) && !strings.HasPrefix(a.text, "--") && strings.Contains(a.text, "R"))
}

// chmodPaths: the first word that is not an option is the mode unless
// --reference is given; the others are written, subtrees with -R.
func chmodPaths(w *pathWalk, args []word, sc scope) {
	args = args[1:]
	tree := slices.ContainsFunc(args, chmodRecursive)
	mode, opts := true, true
	for _, a := range args {
		ref, isRef := strings.CutPrefix(a.text, "--reference=")
		switch {
		case opts && a.text == "--":
			opts = false
		case opts && isRef:
			w.path(valueWord(ref), AccessRead, false, sc)
			mode = false
		case opts && chmodOption(a.text):
		case mode:
			mode = false
		default:
			w.path(a, AccessWrite, tree, sc)
		}
	}
}

var (
	findOuts  = []string{"-fprint", "-fprint0", "-fprintf", "-fls"}
	findFirst = []string{"-H", "-L", "-P"}
)

// findPaths: the roots (the cwd without any) are read subtrees, written with
// -delete; -fprint files are written; -exec commands are walked, and one
// that takes the found names ("{}") is unresolved.
func findPaths(w *pathWalk, args []word, sc scope) {
	args = args[1:]
	i := 0
	for i < len(args) && (slices.Contains(findFirst, args[i].text) || strings.HasPrefix(args[i].text, "-O")) {
		i++
	}
	end := i + slices.IndexFunc(args[i:], func(a word) bool { return strings.IndexByte("-()!,", firstByte(a.text)) >= 0 })
	if end < i {
		end = len(args)
	}
	a := w.findExpr(args[end:], sc)
	if end == i {
		w.cwdRef(a, true, sc)
	}
	for _, root := range args[i:end] {
		w.path(root, a, true, sc)
	}
}

func firstByte(s string) byte {
	if s == "" {
		return 0
	}
	return s[0]
}

// findExpr walks the expression and returns the access to the roots.
func (w *pathWalk) findExpr(expr []word, sc scope) Access {
	a := AccessRead
	for j := 0; j < len(expr); j++ {
		t := expr[j].text
		switch {
		case t == "-delete":
			a = AccessWrite
		case slices.Contains(findOuts, t) && j+1 < len(expr):
			w.path(expr[j+1], AccessWrite, false, sc)
		case slices.Contains(findExec, t):
			end := execEnd(expr, j+1)
			w.findExec(t, expr[j+1:end], sc)
			j = end
		}
	}
	return a
}

func (w *pathWalk) findExec(action string, cmd []word, sc scope) {
	if len(cmd) == 0 || slices.ContainsFunc(cmd, func(a word) bool { return strings.Contains(a.text, "{}") }) {
		w.unresolved = true
		return
	}
	if strings.HasSuffix(action, "dir") {
		sc.cwd = ""
	}
	sc.depth++
	w.call(cmd, sc)
}

var (
	tarSpec = optSpec{value: "fCbgHIKLNTVX", loose: true, long: longs("file directory files-from exclude-from exclude " +
		"listed-incremental use-compress-program to-command transform xform owner group mode mtime newer " +
		"newer-mtime after-date format blocking-factor record-size label")}
	tarCreate  = []string{"-c", "-r", "-u", "-A", "--create", "--append", "--update", "--catenate", "--concatenate", "--delete"}
	tarExtract = []string{"-x", "--extract", "--get"}
	tarList    = []string{"-t", "-d", "--list", "--diff", "--compare"}
	tarLists   = map[string]Access{"-T": AccessRead, "-X": AccessRead, "--files-from": AccessRead,
		"--exclude-from": AccessRead, "-g": AccessWrite, "--listed-incremental": AccessWrite}
)

// tarPaths: create reads its operands (from -C) as subtrees and writes the
// archive; extract reads the archive and writes -C (or the cwd) as a subtree;
// list reads the archive; no known mode is unresolved.
func tarPaths(w *pathWalk, args []word, sc scope) {
	args, ok := tarArgs(args[1:])
	opts, ops, scanned := scanRole(args, tarSpec)
	if !ok || !scanned {
		w.unresolved = true
		return
	}
	w.optPaths(opts, tarLists, sc)
	dir := w.dir(opts, []string{"-C", "--directory"}, sc)
	switch {
	case hasOpt(opts, tarExtract):
		w.archive(opts, AccessRead, sc)
		w.cwdRef(AccessWrite, true, dir)
	case hasOpt(opts, tarCreate):
		w.archive(opts, AccessWrite, sc)
		for _, o := range ops {
			w.path(o, AccessRead, true, dir)
		}
	case hasOpt(opts, tarList):
		w.archive(opts, AccessRead, sc)
	default:
		w.unresolved = true
	}
}

// tarArgs turns an old-style first cluster ("czf") into "-czf"; its one
// value letter moves last. More value letters are not supported (ok false).
func tarArgs(args []word) ([]word, bool) {
	if len(args) == 0 || strings.HasPrefix(args[0].text, "-") {
		return args, true
	}
	t := args[0].text
	i := strings.IndexAny(t, tarSpec.value)
	if i >= 0 && strings.ContainsAny(t[i+1:], tarSpec.value) {
		return nil, false
	}
	if i >= 0 {
		t = t[:i] + t[i+1:] + t[i:i+1]
	}
	out := slices.Clone(args)
	out[0] = word{text: "-" + t, lit: args[0].lit}
	return out, true
}

// archive adds the -f/--file archives; "-" is a stream.
func (w *pathWalk) archive(opts []opt, a Access, sc scope) {
	for _, o := range opts {
		if o.matches("--file") || o.name == "-f" {
			if o.val.text != "-" {
				w.path(o.val, a, false, sc)
			}
		}
	}
}

// dir returns the scope with the cwd set by a folder option: none keeps it,
// one sets it, more make it unknown.
func (w *pathWalk) dir(opts []opt, names []string, sc scope) scope {
	var vals []word
	for _, o := range opts {
		if slices.ContainsFunc(names, o.matches) {
			vals = append(vals, o.val)
		}
	}
	switch len(vals) {
	case 0:
		return sc
	case 1:
		p, _, ok := sc.locate(vals[0])
		if !ok {
			w.unresolved = true
		}
		sc.cwd = p
		return sc
	}
	sc.cwd = ""
	return sc
}

// ddPaths: if= is read, of= is written.
func ddPaths(w *pathWalk, args []word, sc scope) {
	for _, a := range args[1:] {
		k, v, _ := strings.Cut(a.text, "=")
		switch k {
		case "if":
			w.path(valueWord(v), AccessRead, false, sc)
		case "of":
			w.path(valueWord(v), AccessWrite, false, sc)
		}
	}
}

var unzipSpec = optSpec{value: "dP", loose: true}

// unzipPaths: the archive is read; -d (or the cwd) is written as a subtree.
func unzipPaths(w *pathWalk, args []word, sc scope) {
	opts, ops, ok := scanRole(args[1:], unzipSpec)
	if !ok {
		w.unresolved = true
		return
	}
	if len(ops) > 0 {
		w.path(ops[0], AccessRead, false, sc)
	}
	w.cwdRef(AccessWrite, true, w.dir(opts, []string{"-d"}, sc))
}

// copyRole describes scp and rsync: a server side reads or writes its last
// operand; a client reads its local operands and writes the last one. A
// written operand may be a folder the files go into, so it is a subtree.
type copyRole struct {
	spec                   optSpec
	reads, writes, recurse []string
	opts                   map[string]Access
}

func (r copyRole) paths(w *pathWalk, args []word, sc scope) {
	opts, ops, ok := scanRole(args[1:], r.spec)
	if !ok {
		w.unresolved = true
		return
	}
	w.optPaths(opts, r.opts, sc)
	tree := hasOpt(opts, r.recurse)
	switch {
	case len(ops) > 0 && hasOpt(opts, r.reads):
		w.path(ops[len(ops)-1], AccessRead, tree, sc)
	case len(ops) > 0 && hasOpt(opts, r.writes):
		w.path(ops[len(ops)-1], AccessWrite, true, sc)
	default:
		copyClient(w, ops, tree, sc)
	}
}

// copyClient reads the local operands and writes the last one; remote operands are skipped.
func copyClient(w *pathWalk, ops []word, tree bool, sc scope) {
	for i, o := range ops {
		if _, remote := remoteHost(o.text); remote {
			continue
		}
		if i == len(ops)-1 && i > 0 {
			w.path(o, AccessWrite, true, sc)
		} else {
			w.path(o, AccessRead, tree, sc)
		}
	}
}

// shellPaths walks a "-c" command string; any other form reads its path-like words.
func shellPaths(w *pathWalk, args []word, sc scope) {
	if cmd, ok := shellCommand(args[1:]); ok {
		w.script(cmd, sc)
		return
	}
	w.generic(args, sc)
}

// suPaths walks the -c command string as another user; a login form runs it in another folder.
func suPaths(w *pathWalk, args []word, sc scope) {
	opts, operands, ok := scanOpts(args[1:], suSpec, false)
	if !ok {
		w.unresolved = true
		return
	}
	if hasOpt(opts, []string{"-l", "--login"}) || slices.ContainsFunc(operands, func(o word) bool { return o.text == "-" }) {
		sc.cwd = ""
	}
	sc.priv = true
	for _, o := range opts {
		if o.name == "-c" || o.name == "--command" || o.name == "--session-command" {
			w.script(o.val, sc)
		}
	}
}
