package shellclass

import (
	"errors"
	"path"
	"slices"
	"strings"
)

// PathRule is a compiled folder rule: Effect "only", "deny" or "approval".
type PathRule struct {
	ID     string
	Globs  []string
	Access Access
	Effect string
}

// segments splits "/a/b" into ["", "a", "b"], "/" into [""] and "~/a" into ["~", "a"].
func segments(p string) []string {
	if p == "/" {
		return []string{""}
	}
	return strings.Split(p, "/")
}

// matchRows runs the segment DP of glob g over path p: row i holds, for each
// prefix length j of p, whether g[:i] matches p[:j]. It returns whether the
// whole glob matches the whole path and whether some glob prefix matches the
// whole path (a descendant-or-self of p can match g). Cost: |g| × |p|.
func matchRows(g, p []string) (full, prefix bool) {
	prev := make([]bool, len(p)+1)
	cur := make([]bool, len(p)+1)
	prev[0] = true
	for _, seg := range g {
		cur[0] = seg == "**" && prev[0]
		for j := 1; j <= len(p); j++ {
			if seg == "**" {
				cur[j] = prev[j] || cur[j-1]
				continue
			}
			ok, _ := path.Match(seg, p[j-1])
			cur[j] = ok && prev[j-1]
		}
		prev, cur = cur, prev
		prefix = prefix || prev[len(p)]
	}
	return prev[len(p)], prefix
}

// MatchPath reports whether the normalized path p matches glob: "*" within one
// segment, "**" any number of segments (zero included), "~" only the leading
// home segment. A path outside the literal prefix of glob is rejected before
// the segment DP.
func MatchPath(glob, p string) bool {
	return strings.HasPrefix(p, literalPrefix(glob)) && matchFull(glob, p)
}

// matchPrefix reports whether p or some path under p may match glob (Ruling G5).
// p must lie on the literal prefix of glob or under it.
func matchPrefix(glob, p string) bool {
	l := literalPrefix(glob)
	return (strings.HasPrefix(p, l) || strings.HasPrefix(l, p)) && matchSubtree(glob, p)
}

// matchFull is MatchPath without the literal prefilter.
func matchFull(glob, p string) bool {
	full, _ := matchRows(segments(glob), segments(p))
	return full
}

// matchSubtree is matchPrefix without the literal prefilter.
func matchSubtree(glob, p string) bool {
	_, prefix := matchRows(segments(glob), segments(p))
	return prefix
}

// globMeta are the bytes path.Match treats as special; "**" contains "*".
const globMeta = `*?[\`

// literalPrefix is the whole segments of glob before its first segment with
// a special byte ("/a/b" of "/a/b/*.c"; all of a glob without one). Those
// segments only match themselves, so every path a glob matches starts with
// it, and every path a glob prefix matches is on it or under it: the
// prefilter only rejects pairs the segment DP would reject.
func literalPrefix(glob string) string {
	i := strings.IndexAny(glob, globMeta)
	if i < 0 {
		return glob
	}
	return glob[:max(strings.LastIndexByte(glob[:i], '/'), 0)]
}

const errGlob = "invalid path glob: "

// ValidPathGlob checks a folder rule glob: 1–1024 bytes without control
// characters, absolute, "~" or "~/…", clean, "**" only as a whole segment,
// "~" only as the first segment, and every segment a valid path.Match pattern.
func ValidPathGlob(glob string) error {
	switch {
	case glob == "" || len(glob) > maxPathLen:
		return errors.New(errGlob + "length must be 1-1024 bytes")
	case !clean(glob):
		return errors.New(errGlob + "control character or invalid UTF-8")
	case !strings.HasPrefix(glob, "/") && glob != "~" && !strings.HasPrefix(glob, "~/"):
		return errors.New(errGlob + "must start with / or ~")
	case path.Clean(glob) != glob:
		return errors.New(errGlob + "must be clean (no ., .., // or trailing /)")
	}
	for i, seg := range segments(glob) {
		if err := validSegment(i, seg); err != nil {
			return err
		}
	}
	return nil
}

func validSegment(i int, seg string) error {
	switch {
	case seg != "**" && strings.Contains(seg, "**"):
		return errors.New(errGlob + "** must be a whole segment")
	case i > 0 && strings.HasPrefix(seg, "~"), i == 0 && seg != "" && seg != "~":
		return errors.New(errGlob + "~ only as the first segment")
	}
	if _, err := path.Match(seg, ""); err != nil {
		return errors.New(errGlob + "bad pattern")
	}
	return nil
}

// EvalPaths matches refs against rules. hits are the deny and approval rules
// a ref of a matching access hits; outside are the only rules a ref of a
// matching access is not covered by. unresolved puts every only rule outside;
// deny and approval still hit on the resolved refs. Both lists are sorted
// without repeats.
func EvalPaths(rules []PathRule, refs []PathRef, unresolved bool) (hits, outside []string) {
	for _, r := range rules {
		switch {
		case r.Effect == "only" && (unresolved || !covers(r, refs)):
			outside = append(outside, r.ID)
		case (r.Effect == "deny" || r.Effect == "approval") && hitsAny(r, refs):
			hits = append(hits, r.ID)
		}
	}
	slices.Sort(hits)
	slices.Sort(outside)
	return slices.Compact(hits), slices.Compact(outside)
}

func applies(r PathRule, ref PathRef) bool {
	return r.Access == AccessAny || r.Access == ref.Access
}

// hitsAny: a plain ref matches a glob; a subtree ref hits when a path under it may match.
func hitsAny(r PathRule, refs []PathRef) bool {
	for _, ref := range refs {
		if !applies(r, ref) {
			continue
		}
		match := MatchPath
		if ref.Subtree {
			match = matchPrefix
		}
		if slices.ContainsFunc(r.Globs, func(g string) bool { return match(g, ref.Path) }) {
			return true
		}
	}
	return false
}

// covers: every ref of a matching access is covered by a glob; a subtree ref
// only by a glob ending in "/**" that matches its root.
func covers(r PathRule, refs []PathRef) bool {
	for _, ref := range refs {
		if applies(r, ref) && !slices.ContainsFunc(r.Globs, func(g string) bool {
			return (!ref.Subtree || strings.HasSuffix(g, "/**")) && MatchPath(g, ref.Path)
		}) {
			return false
		}
	}
	return true
}

// ExpandHome returns a copy of rules with "~" and "~/…" globs resolved from
// home; a home that is not an absolute clean path keeps "~".
func ExpandHome(rules []PathRule, home string) []PathRule {
	out := slices.Clone(rules)
	for i := range out {
		out[i].Globs = slices.Clone(out[i].Globs)
		for j, g := range out[i].Globs {
			out[i].Globs[j] = expandTilde(g, home)
		}
	}
	return out
}
