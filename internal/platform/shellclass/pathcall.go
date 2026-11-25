package shellclass

import (
	"slices"
	"strings"
)

// call adds the paths of one simple command and returns the cwd after it.
// Wrappers are stripped one per depth level as in the class pass; a command
// run by a wrapper other than "command" is a separate process and keeps the
// shell's cwd.
func (w *pathWalk) call(args []word, sc scope) string {
	outer, inShell := sc.cwd, true
	for ; sc.depth <= maxDepth; sc.depth++ {
		w.argv0(args[0], sc)
		name := normalizeName(args[0].text)
		wr, isWrapper := wrappers[name]
		if !args[0].plain() || !isWrapper {
			out := w.run(name, args, sc)
			if inShell {
				return out
			}
			return outer
		}
		rest, s, done := wr.unwrap(args[1:])
		if done {
			if s&opaque != 0 {
				w.unresolved = true
			}
			return outer
		}
		if name == "xargs" {
			w.unresolved = true // the arguments come from the input
			return outer
		}
		inShell = inShell && name == "command"
		sc = wrapped(name, wr, args[1:], sc)
		args = rest
	}
	w.unresolved = true
	return ""
}

// argv0 reads a command named by a path (Ruling G4).
func (w *pathWalk) argv0(a word, sc scope) {
	if strings.Contains(a.text, "/") {
		w.path(a, AccessRead, false, sc)
	}
}

// chdirOpts are wrapper options that run the command in another folder (Ruling G3).
var chdirOpts = map[string][]string{
	"env":  {"-C", "--chdir"},
	"sudo": {"-D", "--chdir", "-i", "--login"},
}

// wrapped returns the scope of the command a wrapper runs; pkexec runs it in
// the target user's home.
func wrapped(name string, wr wrapper, args []word, sc scope) scope {
	opts, _, _ := scanOpts(args, wr.spec, true)
	if name == "pkexec" || hasOpt(opts, chdirOpts[name]) {
		sc.cwd = ""
	}
	sc.priv = sc.priv || wr.adds&privileged != 0
	return sc
}

// run adds the paths of a command that is not a wrapper.
func (w *pathWalk) run(name string, args []word, sc scope) string {
	r, known := pathRoles[name]
	switch {
	case !args[0].plain():
		w.generic(args, sc)
	case name == "cd":
		return w.cd(args[1:], sc)
	case slices.Contains(shellState, name):
		w.unresolved = true
		w.generic(args, sc)
		return ""
	case slices.Contains(cwdMovers, name):
		return ""
	case !known:
		w.generic(args, sc)
	case r.check != nil:
		r.check(w, args, sc)
	default:
		r.eval(w, args, sc)
	}
	return sc.cwd
}

// generic reads every path-like word and every path-like --name=VALUE value (Ruling G4).
func (w *pathWalk) generic(args []word, sc scope) {
	for _, a := range args[1:] {
		name, v, eq := strings.Cut(a.text, "=")
		switch {
		case pathLike(a.text):
			w.path(a, AccessRead, false, sc)
		case eq && strings.HasPrefix(name, "--") && pathLike(v):
			w.path(valueWord(v), AccessRead, false, sc)
		}
	}
}

// cd returns the cwd a literal cd sets. No argument is "~"; "-", options, a
// relative name without "./" or "../" (CDPATH) or a glob make it unknown.
func (w *pathWalk) cd(ops []word, sc scope) string {
	if len(ops) > 0 && ops[0].text == "--" {
		ops = ops[1:]
	}
	if len(ops) == 0 {
		if sc.noTilde {
			return ""
		}
		return "~"
	}
	if len(ops) > 1 || !cdTarget(ops[0]) {
		return ""
	}
	p, _, ok := sc.locate(ops[0])
	if !ok {
		w.unresolved = true
	}
	return p
}

// cdTarget reports a cd argument whose folder follows from the text alone;
// a non-literal or tilde one is resolved to mark a failure unresolved.
func cdTarget(a word) bool {
	switch {
	case a.glob || strings.HasPrefix(a.text, "-"):
		return false
	case !a.lit || a.tilde != tildeNone:
		return true
	}
	return strings.HasPrefix(a.text, "/") || dotted(a.text)
}

func dotted(s string) bool {
	return s == "." || s == ".." || strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../")
}

// locate resolves a path word; glob reports a pattern cut to its folder.
func (sc scope) locate(x word) (p string, glob, ok bool) {
	if !x.lit || x.tilde == tildeBad || !clean(x.text) || strings.Contains(x.text, "=~") {
		return "", false, false
	}
	text := x.text
	if x.glob {
		text = globDir(text)
	}
	p, ok = sc.resolve(text, x.tilde == tildeHome)
	return p, x.glob, ok
}

// path adds the path a word names, or marks the walk unresolved.
func (w *pathWalk) path(x word, a Access, tree bool, sc scope) {
	p, glob, ok := sc.locate(x)
	switch {
	case !ok:
		w.unresolved = true
	case !tree && !glob && slices.Contains(streams, p):
	default:
		w.add(PathRef{Path: p, Access: a, Subtree: tree || glob})
	}
}

// cwdRef adds the cwd itself, for commands that act on it without operands.
func (w *pathWalk) cwdRef(a Access, tree bool, sc scope) {
	if sc.cwd == "" || sc.noRel {
		w.unresolved = true
		return
	}
	w.add(PathRef{Path: sc.cwd, Access: a, Subtree: tree})
}

func (w *pathWalk) add(r PathRef) {
	if !w.refs.add(r) {
		w.unresolved = true
	}
}

// optPaths adds the values of path-valued options and path-like long option values.
func (w *pathWalk) optPaths(opts []opt, named map[string]Access, sc scope) {
	for _, o := range opts {
		if o.val.text == "" {
			continue
		}
		if a, ok := optAccess(named, o); ok {
			w.path(o.val, a, false, sc)
		} else if strings.HasPrefix(o.name, "--") && pathLike(o.val.text) {
			w.path(o.val, AccessRead, false, sc)
		}
	}
}

// script walks a "-c" command string one nesting level down. Under a
// privilege wrapper "~" belongs to another user and is unresolved (Ruling G3).
func (w *pathWalk) script(cmd word, sc scope) {
	if !cmd.lit {
		w.unresolved = true
		return
	}
	f, ok := parse(cmd.text, sc.depth+1)
	if !ok {
		w.unresolved = true
		return
	}
	sc.depth++
	sc.noTilde = sc.noTilde || sc.priv
	w.file(f, sc)
}
