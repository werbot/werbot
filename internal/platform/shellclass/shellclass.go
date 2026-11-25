// Package shellclass sorts a shell command line into risk classes (read,
// write, destructive, privilege, secrets, egress, opaque). It parses the
// command with mvdan.cc/sh and walks the syntax tree; anything it cannot
// parse or does not recognise is opaque (fail-closed).
package shellclass

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// Class is a risk class of a shell command.
type Class string

// The seven classes.
const (
	Read        Class = "read"
	Write       Class = "write"
	Destructive Class = "destructive"
	Privilege   Class = "privilege"
	Secrets     Class = "secrets"
	Egress      Class = "egress"
	Opaque      Class = "opaque"
)

const (
	maxCommand = 64 << 10 // longer commands are opaque without parsing
	maxDepth   = 8        // deeper wrapper or "-c" nesting is opaque
)

// classOrder lists the classes in ascending order; bit i of a set stands for classOrder[i].
var classOrder = [...]Class{Destructive, Egress, Opaque, Privilege, Read, Secrets, Write}

// set is a bit set of classes.
type set uint8

func of(cs ...Class) set {
	var s set
	for _, c := range cs {
		s |= 1 << slices.Index(classOrder[:], c)
	}
	return s
}

func (s set) list() []Class {
	out := make([]Class, 0, len(classOrder))
	for i, c := range classOrder {
		if s&(1<<i) != 0 {
			out = append(out, c)
		}
	}
	return out
}

// Classes lists the seven classes in ascending order.
func Classes() []Class { return slices.Clone(classOrder[:]) }

// Valid reports whether c is one of the seven classes.
func Valid(c Class) bool { return slices.Contains(classOrder[:], c) }

// Override assigns classes to commands whose normalized argv0 (and, after a
// space, leading arguments) match the glob Pattern field by field.
type Override struct {
	Pattern string
	Classes []Class
}

type override struct {
	pattern string
	fields  []string
	classes set
}

// Classifier classifies commands with the built-in table and the overrides.
// It is safe for concurrent use.
type Classifier struct {
	overrides []override // more fields first, then by pattern
}

// New checks and orders the overrides. A blank or malformed pattern, or an
// empty or unknown class, is an error.
func New(overrides []Override) (*Classifier, error) {
	c := &Classifier{}
	for _, o := range overrides {
		r, err := compileOverride(o)
		if err != nil {
			return nil, err
		}
		c.overrides = append(c.overrides, r)
	}
	slices.SortFunc(c.overrides, func(a, b override) int {
		if n := len(b.fields) - len(a.fields); n != 0 {
			return n
		}
		return strings.Compare(a.pattern, b.pattern)
	})
	return c, nil
}

func compileOverride(o Override) (override, error) {
	fields := strings.Fields(o.Pattern)
	if len(fields) == 0 {
		return override{}, errors.New("shellclass: empty pattern")
	}
	for _, f := range fields {
		if _, err := path.Match(f, ""); err != nil {
			return override{}, fmt.Errorf("shellclass: pattern %q: %w", o.Pattern, err)
		}
	}
	if len(o.Classes) == 0 {
		return override{}, fmt.Errorf("shellclass: pattern %q has no classes", o.Pattern)
	}
	for _, cl := range o.Classes {
		if !Valid(cl) {
			return override{}, fmt.Errorf("shellclass: pattern %q: unknown class %q", o.Pattern, cl)
		}
	}
	return override{pattern: o.Pattern, fields: fields, classes: of(o.Classes...)}, nil
}

// match returns the classes of the first override that matches the command.
func (c *Classifier) match(name string, args []word) (set, bool) {
	for _, o := range c.overrides {
		if o.matches(name, args) {
			return o.classes, true
		}
	}
	return 0, false
}

func (o override) matches(name string, args []word) bool {
	if len(args) < len(o.fields) {
		return false
	}
	for i, f := range o.fields {
		text := name
		if i > 0 {
			text = args[i].text
		}
		if ok, _ := path.Match(f, text); !ok || !args[i].lit {
			return false
		}
	}
	return true
}

// Classify returns the classes of command, sorted and never empty.
// resourceHost is the host of the target resource: network clients that only
// reach it or the loopback are read, not egress.
func (c *Classifier) Classify(command, resourceHost string) []Class {
	return c.Analyze(command, resourceHost, "~").Classes
}

// script classifies a whole command line at the given nesting depth.
func (c *Classifier) script(src, host string, depth int) set {
	f, ok := parse(src, depth)
	if !ok {
		return of(Opaque)
	}
	return c.classes(f, host, depth)
}

// classes classifies a parsed command line.
func (c *Classifier) classes(f *syntax.File, host string, depth int) set {
	var out set
	calls := 0
	for n := range syntax.Preorder(f) {
		if call, ok := n.(*syntax.CallExpr); ok && len(call.Args) > 0 {
			calls++
			out |= c.call(words(call.Args), host, depth)
			continue
		}
		out |= nodeClasses(n, host)
	}
	if calls == 0 {
		return of(Opaque)
	}
	return out
}

func parse(src string, depth int) (*syntax.File, bool) {
	if depth > maxDepth || len(src) > maxCommand || strings.TrimSpace(src) == "" {
		return nil, false
	}
	f, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(src), "")
	return f, err == nil
}

// hijackVars change how later commands are found or loaded.
var hijackVars = []string{"PATH", "LD_PRELOAD", "LD_LIBRARY_PATH", "BASH_ENV", "ENV", "IFS"}

func hijack(name string) set {
	if slices.Contains(hijackVars, name) {
		return of(Opaque)
	}
	return 0
}

// nodeClasses adds the classes of a syntax node other than a command call.
// Nodes outside the allowlist are opaque; their children are still walked.
func nodeClasses(n syntax.Node, host string) set {
	switch n := n.(type) {
	case *syntax.Redirect:
		return redirect(n, host)
	case *syntax.Assign:
		if n.Name != nil {
			return hijack(n.Name.Value)
		}
		return 0
	case *syntax.ParamExp:
		return paramAssign(n)
	case *syntax.SglQuoted:
		if n.Dollar {
			return of(Opaque)
		}
		return 0
	case *syntax.File, *syntax.Stmt, *syntax.CallExpr, *syntax.Word, *syntax.Lit, *syntax.DblQuoted,
		*syntax.CmdSubst, *syntax.ProcSubst, *syntax.BinaryCmd, *syntax.Subshell, *syntax.Block,
		*syntax.IfClause, *syntax.WhileClause, *syntax.ForClause, *syntax.WordIter, *syntax.CaseClause,
		*syntax.CaseItem, *syntax.TimeClause, *syntax.Comment, nil:
		return 0
	}
	return of(Opaque)
}

// paramAssign flags ${NAME=value} and ${NAME:=value} on a hijack variable.
func paramAssign(p *syntax.ParamExp) set {
	if p.Exp == nil || p.Param == nil {
		return 0
	}
	if p.Exp.Op == syntax.AssignUnset || p.Exp.Op == syntax.AssignUnsetOrNull {
		return hijack(p.Param.Value)
	}
	return 0
}

// quietTargets are output redirection targets that write no file.
var quietTargets = []string{"/dev/null", "/dev/stdout", "/dev/stderr", "/dev/fd/1", "/dev/fd/2"}

var heredocs = []syntax.RedirOperator{syntax.Hdoc, syntax.DashHdoc, syntax.WordHdoc}

func redirect(r *syntax.Redirect, host string) set {
	if slices.Contains(heredocs, r.Op) {
		return 0
	}
	w := newWord(r.Word)
	if h, ok := netDevice(w.text); ok {
		return pathClasses(w.text) | hostClass([]string{h}, host)
	}
	return pathClasses(w.text) | target(r.Op, w)
}

// target returns the classes of writing to a redirection target.
func target(op syntax.RedirOperator, w word) set {
	switch {
	case (op == syntax.DplOut || op == syntax.DplIn) && fdTarget(w.text):
		return 0
	case op == syntax.RdrIn || op == syntax.DplIn:
		return 0
	case w.lit && slices.Contains(quietTargets, w.text):
		return 0
	}
	return of(Write) | sudoers(w.text)
}

// fdTarget reports a file descriptor duplication or close (">&2", ">&-").
func fdTarget(s string) bool {
	return s == "-" || (s != "" && strings.Trim(s, "0123456789") == "")
}
