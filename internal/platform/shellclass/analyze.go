package shellclass

import (
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// Analyze classifies command like Classify and lists the paths it touches,
// normalized from cwd ("~", "~/…" or absolute; any other value makes relative
// paths unresolved). A path that cannot be computed sets Unresolved and adds
// opaque to the classes. The class pass and the path pass walk the same parse.
func (c *Classifier) Analyze(command, resourceHost, cwd string) Analysis {
	f, ok := parse(command, 0)
	if !ok {
		return Analysis{Classes: opaque.list(), Unresolved: true}
	}
	s := c.classes(f, resourceHost, 0)
	var w pathWalk
	w.file(f, scope{cwd: startCwd(cwd)})
	if w.unresolved {
		s |= opaque
	}
	return Analysis{Classes: s.list(), Paths: w.refs.sorted(), Unresolved: w.unresolved}
}

// pathWalk collects the paths of a command line. The cwd is tracked through
// the statement structure (Ruling C3): a cd sets it only for the right side
// of "&&" (Ruling C4); any other change makes it unknown.
type pathWalk struct {
	refs       refSet
	unresolved bool
}

func (w *pathWalk) file(f *syntax.File, sc scope) {
	if setsHome(f) {
		sc.noTilde, sc.noRel = true, true
	}
	w.seq(f.Stmts, sc)
}

// seq walks a statement list and returns the cwd after its last statement;
// a statement that changes the cwd makes it unknown for the next ones.
func (w *pathWalk) seq(stmts []*syntax.Stmt, sc scope) string {
	out := sc.cwd
	for _, s := range stmts {
		out = w.stmt(s, sc)
		if out != sc.cwd {
			sc.cwd = ""
		}
	}
	return out
}

func (w *pathWalk) stmt(s *syntax.Stmt, sc scope) string {
	for _, r := range s.Redirs {
		w.redirect(r, sc)
	}
	out := w.command(s.Cmd, sc)
	if out != sc.cwd && (s.Negated || s.Background || s.Coprocess || s.Disown) {
		return ""
	}
	return out
}

func (w *pathWalk) command(cmd syntax.Command, sc scope) string {
	switch x := cmd.(type) {
	case nil:
		return sc.cwd
	case *syntax.CallExpr:
		return w.callExpr(x, sc)
	case *syntax.BinaryCmd:
		return w.binary(x, sc)
	case *syntax.Subshell:
		w.seq(x.Stmts, sc)
		return sc.cwd
	case *syntax.Block:
		return w.seq(x.Stmts, sc)
	case *syntax.TimeClause:
		if x.Stmt == nil {
			return sc.cwd
		}
		return w.stmt(x.Stmt, sc)
	}
	return w.clause(cmd, sc)
}

// binary: only "&&" carries a cwd change to its right side.
func (w *pathWalk) binary(b *syntax.BinaryCmd, sc scope) string {
	mid := w.stmt(b.X, sc)
	right := sc
	right.cwd = mid
	if b.Op == syntax.AndStmt {
		return w.stmt(b.Y, right)
	}
	if mid != sc.cwd {
		right.cwd = ""
	}
	if w.stmt(b.Y, right) != right.cwd || mid != sc.cwd {
		return ""
	}
	return sc.cwd
}

// clause walks if, while, for and case; a cwd change inside makes the cwd
// unknown for the whole clause, which is walked again with it unknown. Any
// other command is unresolved.
func (w *pathWalk) clause(cmd syntax.Command, sc scope) string {
	lists, words, ok := clauseParts(cmd)
	if !ok {
		w.unresolved = true
		return ""
	}
	for _, x := range words {
		w.substs(x, sc)
	}
	if w.lists(lists, sc) {
		return sc.cwd
	}
	sc.cwd = ""
	w.lists(lists, sc)
	return ""
}

// lists walks statement lists; it reports false when one changes the cwd.
func (w *pathWalk) lists(lists [][]*syntax.Stmt, sc scope) bool {
	for _, l := range lists {
		if w.seq(l, sc) != sc.cwd {
			return false
		}
	}
	return true
}

func clauseParts(cmd syntax.Command) (lists [][]*syntax.Stmt, words []*syntax.Word, ok bool) {
	switch x := cmd.(type) {
	case *syntax.IfClause:
		for ; x != nil; x = x.Else {
			lists = append(lists, x.Cond, x.Then)
		}
	case *syntax.WhileClause:
		lists = append(lists, x.Cond, x.Do)
	case *syntax.ForClause:
		iter, isIter := x.Loop.(*syntax.WordIter)
		if !isIter {
			return nil, nil, false
		}
		return [][]*syntax.Stmt{x.Do}, iter.Items, true
	case *syntax.CaseClause:
		words = append(words, x.Word)
		for _, item := range x.Items {
			words = append(words, item.Patterns...)
			lists = append(lists, item.Stmts)
		}
	default:
		return nil, nil, false
	}
	return lists, words, true
}

func (w *pathWalk) callExpr(x *syntax.CallExpr, sc scope) string {
	for _, a := range x.Assigns {
		w.substs(a, sc)
	}
	for _, a := range x.Args {
		w.substs(a, sc)
	}
	if len(x.Args) == 0 {
		return sc.cwd
	}
	return w.call(words(x.Args), sc)
}

// substs walks the command and process substitutions inside n as subshells.
func (w *pathWalk) substs(n syntax.Node, sc scope) {
	syntax.Walk(n, func(n syntax.Node) bool {
		switch n := n.(type) {
		case *syntax.CmdSubst:
			w.seq(n.Stmts, sc)
			return false
		case *syntax.ProcSubst:
			w.seq(n.Stmts, sc)
			return false
		}
		return true
	})
}

// redirect: ">", ">>", ">|", "&>", "&>>", "<>", ">&file" write; "<" reads;
// here-documents and descriptor duplications name no path.
func (w *pathWalk) redirect(r *syntax.Redirect, sc scope) {
	if r.Hdoc != nil {
		w.substs(r.Hdoc, sc)
	}
	if r.Word == nil {
		return
	}
	w.substs(r.Word, sc)
	x := newWord(r.Word)
	dup := r.Op == syntax.DplOut || r.Op == syntax.DplIn
	if slices.Contains(heredocs, r.Op) || (dup && fdTarget(x.text)) {
		return
	}
	a := AccessWrite
	if r.Op == syntax.RdrIn || r.Op == syntax.DplIn {
		a = AccessRead
	}
	w.path(x, a, false, sc)
}

// homeVars change what "~" and relative cd arguments mean (Ruling G3).
var homeVars = []string{"HOME", "CDPATH"}

// setsHome reports a possible assignment of HOME or CDPATH anywhere in f: a
// prefix or bare assignment, a declaration, ${HOME:=…}, a NAME=VALUE
// argument, or the bare name as an argument (read, printf -v, unset).
func setsHome(f *syntax.File) bool {
	found := false
	syntax.Walk(f, func(n syntax.Node) bool {
		found = found || homeNode(n)
		return !found
	})
	return found
}

func homeNode(n syntax.Node) bool {
	switch n := n.(type) {
	case *syntax.Assign:
		return n.Name != nil && slices.Contains(homeVars, n.Name.Value)
	case *syntax.ParamExp:
		return n.Param != nil && slices.Contains(homeVars, n.Param.Value) && paramSets(n)
	case *syntax.Word:
		lit, ok := n.Parts[0].(*syntax.Lit)
		return ok && slices.ContainsFunc(homeVars, func(v string) bool {
			return lit.Value == v || strings.HasPrefix(lit.Value, v+"=")
		})
	}
	return false
}

// paramSets reports ${NAME=value} or ${NAME:=value}.
func paramSets(p *syntax.ParamExp) bool {
	return p.Exp != nil && (p.Exp.Op == syntax.AssignUnset || p.Exp.Op == syntax.AssignUnsetOrNull)
}
