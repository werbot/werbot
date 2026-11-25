package shellclass

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// The literal-prefix index of a PathSet only skips globs the segment DP
// rejects: for every ref of the prefilter corpus, alone and all together,
// Eval gives the answers of the linear unfiltered reference.
func TestPathSetIndexEquivalent(t *testing.T) {
	globs, paths := prefilterInputs(t)
	rules := equivalenceRules(globs)
	set := CompilePaths(rules)
	var all []PathRef
	var nonEmpty int
	for _, p := range paths {
		for _, r := range []PathRef{ref(p, AccessRead, false), ref(p, AccessWrite, true)} {
			all = append(all, r)
			if hits, outside := set.Eval([]PathRef{r}, false); evalAgrees(t, rules, []PathRef{r}, false, hits, outside) {
				nonEmpty++
			}
		}
	}
	for _, unresolved := range []bool{false, true} {
		hits, outside := set.Eval(all, unresolved)
		evalAgrees(t, rules, all, unresolved, hits, outside)
	}
	if nonEmpty < len(all)/2 {
		t.Fatalf("%d of %d refs hit a rule: the corpus does not exercise the index", nonEmpty, len(all))
	}
}

// equivalenceRules are, for every valid glob, a rule of each effect and
// access with that glob alone and with the next valid glob too.
func equivalenceRules(globs []string) []PathRule {
	valid := slices.DeleteFunc(slices.Clone(globs), func(g string) bool { return ValidPathGlob(g) != nil })
	var rules []PathRule
	for i, g := range valid {
		for _, effect := range PathEffects() {
			for _, access := range PathAccesses() {
				id := fmt.Sprintf("%s:%s:%d", effect, access, i)
				rules = append(rules,
					PathRule{ID: id, Globs: []string{g}, Access: Access(access), Effect: effect},
					PathRule{ID: id + "+", Globs: []string{g, valid[(i+1)%len(valid)]}, Access: Access(access), Effect: effect})
			}
		}
	}
	return rules
}

// evalAgrees compares hits and outside with evalLinear; it reports whether
// the reference has a hit or an only rule left inside.
func evalAgrees(t *testing.T, rules []PathRule, refs []PathRef, unresolved bool, hits, outside []string) bool {
	t.Helper()
	wantHits, wantOutside := evalLinear(rules, refs, unresolved)
	if !slices.Equal(hits, wantHits) || !slices.Equal(outside, wantOutside) {
		t.Errorf("refs %v unresolved %v: hits %v outside %v, linear %v %v", refs, unresolved, hits, outside, wantHits, wantOutside)
	}
	return len(wantHits) > 0 || len(wantOutside) < len(rules)/3
}

// evalLinear is Eval without the index and the literal prefilter: every
// glob of every rule against every ref through the unfiltered segment DP.
func evalLinear(rules []PathRule, refs []PathRef, unresolved bool) (hits, outside []string) {
	for _, r := range rules {
		switch {
		case r.Effect == EffectOnly && (unresolved || slices.ContainsFunc(refs, func(ref PathRef) bool { return linearApplies(r, ref) && !linearCovers(r, ref) })):
			outside = append(outside, r.ID)
		case r.Effect != EffectOnly && slices.ContainsFunc(refs, func(ref PathRef) bool { return linearApplies(r, ref) && linearHits(r, ref) }):
			hits = append(hits, r.ID)
		}
	}
	slices.Sort(hits)
	slices.Sort(outside)
	return hits, outside
}

func linearApplies(r PathRule, ref PathRef) bool {
	return r.Access == AccessAny || r.Access == ref.Access
}

func linearHits(r PathRule, ref PathRef) bool {
	return slices.ContainsFunc(r.Globs, func(g string) bool {
		return ref.Subtree && matchSubtree(g, ref.Path) || !ref.Subtree && matchFull(g, ref.Path)
	})
}

func linearCovers(r PathRule, ref PathRef) bool {
	return slices.ContainsFunc(r.Globs, func(g string) bool {
		return (!ref.Subtree || strings.HasSuffix(g, "/**")) && matchFull(g, ref.Path)
	})
}
