package shellclass

import (
	"errors"
	"path"
	"slices"
	"strings"
)

// PathRule is a compiled folder rule.
type PathRule struct {
	ID     string
	Globs  []string
	Access Access
	Effect string // EffectOnly, EffectDeny or EffectApproval
}

// The effects of a PathRule.
const (
	EffectOnly     = "only"
	EffectDeny     = "deny"
	EffectApproval = "approval"
)

// PathAccesses returns the valid accesses of a PathRule (a new slice each call).
func PathAccesses() []string {
	return []string{string(AccessRead), string(AccessWrite), string(AccessAny)}
}

// PathEffects returns the valid effects of a PathRule (a new slice each call).
func PathEffects() []string { return []string{EffectOnly, EffectDeny, EffectApproval} }

// Validate checks that r has globs, each one valid, a known access and a known effect.
func (r PathRule) Validate() error {
	if len(r.Globs) == 0 || !slices.Contains(PathAccesses(), string(r.Access)) || !slices.Contains(PathEffects(), r.Effect) {
		return errors.New("invalid paths, access or effect")
	}
	for _, g := range r.Globs {
		if err := ValidPathGlob(g); err != nil {
			return err
		}
	}
	return nil
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

// pathGlob is a valid glob compiled once: its literal prefix, its segments
// and whether it ends in "/**" (it can cover a subtree ref).
type pathGlob struct {
	lit  string
	segs []string
	tree bool
}

func compileGlob(glob string) pathGlob {
	return pathGlob{lit: literalPrefix(glob), segs: segments(glob), tree: strings.HasSuffix(glob, "/**")}
}

// match reports whether the normalized path p matches g: "*" within one
// segment, "**" any number of segments (zero included), "~" only the leading
// home segment. A path outside the literal prefix of g is rejected before
// the segment DP.
func (g *pathGlob) match(p string) bool {
	if !strings.HasPrefix(p, g.lit) {
		return false
	}
	full, _ := matchRows(g.segs, segments(p))
	return full
}

// under reports whether p or some path under p may match g (Ruling G5).
// p must lie on the literal prefix of g or under it.
func (g *pathGlob) under(p string) bool {
	if !strings.HasPrefix(p, g.lit) && !strings.HasPrefix(g.lit, p) {
		return false
	}
	_, prefix := matchRows(g.segs, segments(p))
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
