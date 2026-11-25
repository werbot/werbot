package shellclass

import (
	"slices"
	"strings"
)

// PathSet is path rules with their globs compiled once, for Eval: each glob
// split into segments and indexed by its literal prefix.
type PathSet struct {
	rules []compiledRule
	only  []int     // indexes of the only rules
	lits  []litGlob // every glob, by literal prefix
}

type compiledRule struct {
	PathRule
	globs []pathGlob
}

// litGlob is a glob of rules[rule].
type litGlob struct {
	rule int
	glob *pathGlob
}

// CompilePaths compiles rules; each one must be valid (PathRule.Validate).
func CompilePaths(rules []PathRule) PathSet {
	s := PathSet{rules: make([]compiledRule, len(rules))}
	for i, r := range rules {
		c := compiledRule{PathRule: r, globs: make([]pathGlob, len(r.Globs))}
		for j, g := range r.Globs {
			c.globs[j] = compileGlob(g)
		}
		s.rules[i] = c
		if r.Effect == EffectOnly {
			s.only = append(s.only, i)
		}
	}
	for i := range s.rules {
		for j := range s.rules[i].globs {
			s.lits = append(s.lits, litGlob{i, &s.rules[i].globs[j]})
		}
	}
	slices.SortFunc(s.lits, func(a, b litGlob) int { return strings.Compare(a.glob.lit, b.glob.lit) })
	return s
}

// EvalPaths compiles rules and evaluates refs on them (PathSet.Eval).
func EvalPaths(rules []PathRule, refs []PathRef, unresolved bool) (hits, outside []string) {
	return CompilePaths(rules).Eval(refs, unresolved)
}

// Eval matches refs against the rules of s. hits are the deny and approval
// rules a ref of a matching access hits; outside are the only rules a ref of
// a matching access is not covered by. unresolved puts every only rule
// outside; deny and approval still hit on the resolved refs. Both lists are
// sorted without repeats.
func (s PathSet) Eval(refs []PathRef, unresolved bool) (hits, outside []string) {
	e := evaluation{set: &s, flag: make([]bool, len(s.rules)), covered: make([]int, len(s.rules))}
	for k, ref := range refs {
		e.ref(k+1, ref)
	}
	for i := range s.rules {
		switch r := &s.rules[i]; {
		case r.Effect == EffectOnly && (unresolved || e.flag[i]):
			outside = append(outside, r.ID)
		case r.Effect != EffectOnly && e.flag[i]:
			hits = append(hits, r.ID)
		}
	}
	slices.Sort(hits)
	slices.Sort(outside)
	return slices.Compact(hits), slices.Compact(outside)
}

// evaluation is the state of one Eval: flag marks a deny or approval rule
// hit and an only rule outside; covered is the stamp of the last ref an only
// rule covers.
type evaluation struct {
	set     *PathSet
	flag    []bool
	covered []int
}

// ref matches the ref with stamp against the globs that may match it, then
// puts outside every only rule of its access that no glob covers it by.
func (e *evaluation) ref(stamp int, ref PathRef) {
	e.set.candidates(ref, func(c litGlob) { e.match(stamp, ref, c) })
	for _, i := range e.set.only {
		if e.set.rules[i].applies(ref) && e.covered[i] != stamp {
			e.flag[i] = true
		}
	}
}

// match marks the rule of c hit or covering the ref with stamp by its glob.
func (e *evaluation) match(stamp int, ref PathRef, c litGlob) {
	switch r := &e.set.rules[c.rule]; {
	case e.flag[c.rule] || e.covered[c.rule] == stamp || !r.applies(ref):
	case r.Effect == EffectOnly:
		if c.glob.covers(ref) {
			e.covered[c.rule] = stamp
		}
	case r.Effect == EffectDeny || r.Effect == EffectApproval:
		e.flag[c.rule] = c.glob.hits(ref)
	}
}

// hits: a plain ref matches g; a subtree ref hits when a path under it may match.
func (g *pathGlob) hits(ref PathRef) bool {
	if ref.Subtree {
		return g.under(ref.Path)
	}
	return g.match(ref.Path)
}

// covers: g matches a plain ref; a subtree ref only when g ends in "/**" and matches its root.
func (g *pathGlob) covers(ref PathRef) bool {
	return (!ref.Subtree || g.tree) && g.match(ref.Path)
}

func (r *compiledRule) applies(ref PathRef) bool {
	return r.Access == AccessAny || r.Access == ref.Access
}

// candidates calls f with every glob that may match ref: those whose literal
// prefix is "", a whole-segment prefix of the path or the path itself, and
// for a subtree ref those whose literal prefix starts with the path. A glob
// matches only paths on its literal prefix or under it, segment by segment.
func (s *PathSet) candidates(ref PathRef, f func(litGlob)) {
	p := ref.Path
	for i := range len(p) {
		if i == 0 || p[i] == '/' {
			s.visit(p[:i], false, f)
		}
	}
	s.visit(p, ref.Subtree, f)
}

// visit calls f with the globs whose literal prefix is lit, or starts with it when under.
func (s *PathSet) visit(lit string, under bool, f func(litGlob)) {
	i, _ := slices.BinarySearchFunc(s.lits, lit, func(c litGlob, t string) int { return strings.Compare(c.glob.lit, t) })
	for ; i < len(s.lits) && (s.lits[i].glob.lit == lit || under && strings.HasPrefix(s.lits[i].glob.lit, lit)); i++ {
		f(s.lits[i])
	}
}
