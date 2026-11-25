package shellclass

import (
	"cmp"
	"strings"
	"testing"
)

// prefilterGlobs and prefilterPaths cover every kind of glob segment (none
// special, "*", "**", "?", "[a-z]", escapes, "~", a trailing "/**") and paths
// at, above, below and beside the literal prefix of each glob.
var (
	prefilterGlobs = []string{
		"/", "/**", "~", "~/**", "/srv/app", "/srv/app/x", "/srv/app/**", "/srv/*/secret/**", "/srv/a?p/x",
		"/srv/[a-z]pp/**", `/srv/\*x`, `/srv/a\pp`, `/srv/ap\?/**`, "/srv/**/x", "/srv/*.key", "/*/app",
		"~/app/**", "~/app/*.yml", "~/*", "/etc/**", "/home/*/secret/**", "/srv/app/**/logs/*.log",
	}
	prefilterPaths = []string{
		"/", "/s", "/srv", "/srvx", "/srv/ap", "/srv/app", "/srv/appx", "/srv/apps/x", "/srv/apq", "/srv/app/x",
		"/srv/app/x/y", "/srv/app/secret", "/srv/b/secret/k", "/srv/*x", "/srv/x", "/srv/a.key", "/srv/app/a/logs/b.log",
		"~", "~/ap", "~/app", "~/app/config.yml", "~/app/x/y", "/etc", "/etc/hosts", "/home/u/secret/k", "/x/app",
	}
)

// The literal prefilter of a compiled glob's match and under only rejects
// pairs the segment DP rejects: with and without it every pair gets the same answer.
func TestPathPrefilterEquivalent(t *testing.T) {
	globs, paths := prefilterInputs(t)
	var pairs, rejected int
	for _, g := range globs {
		if ValidPathGlob(g) != nil {
			continue
		}
		for _, p := range paths {
			pairs++
			if !prefilterAgrees(t, g, p) {
				rejected++
			}
		}
	}
	if pairs < 1000 || rejected < pairs/4 {
		t.Fatalf("%d pairs, %d rejected by the prefilter: the corpus does not exercise it", pairs, rejected)
	}
}

// prefilterInputs are prefilterGlobs and prefilterPaths plus the match
// cases and the corpus refs, as paths and as globs.
func prefilterInputs(t *testing.T) (globs, paths []string) {
	t.Helper()
	globs, paths = prefilterGlobs, prefilterPaths
	for _, tc := range matchPathCases {
		globs, paths = append(globs, tc.glob), append(paths, tc.path)
	}
	for _, row := range loadPaths(t) {
		for _, tok := range strings.Fields(row.want) {
			if _, ref, ok := strings.Cut(tok, ":"); ok {
				globs, paths = append(globs, ref), append(paths, cmp.Or(strings.TrimSuffix(ref, "/**"), "/"))
			}
		}
	}
	return globs, paths
}

// prefilterAgrees compares the filtered and unfiltered matches of g and p; it
// reports whether p starts with the literal prefix of g (the prefilter passes it).
func prefilterAgrees(t *testing.T, g, p string) bool {
	t.Helper()
	c := compileGlob(g)
	if got, want := c.match(p), matchFull(g, p); got != want {
		t.Errorf("match(%q, %q) = %v, unfiltered %v", g, p, got, want)
	}
	if got, want := c.under(p), matchSubtree(g, p); got != want {
		t.Errorf("under(%q, %q) = %v, unfiltered %v", g, p, got, want)
	}
	return strings.HasPrefix(p, c.lit)
}

// matchFull is match without the literal prefilter.
func matchFull(glob, p string) bool {
	full, _ := matchRows(segments(glob), segments(p))
	return full
}

// matchSubtree is under without the literal prefilter.
func matchSubtree(glob, p string) bool {
	_, prefix := matchRows(segments(glob), segments(p))
	return prefix
}

func TestLiteralPrefix(t *testing.T) {
	for glob, want := range map[string]string{
		"/":              "/",
		"/**":            "",
		"/srv/app":       "/srv/app",
		"/srv/app/**":    "/srv/app",
		"/srv/*/x":       "/srv",
		`/srv/a\pp/x`:    "/srv",
		"/srv/[ab]/x":    "/srv",
		"/srv/a?/x":      "/srv",
		"~":              "~",
		"~/*":            "~",
		"~/app/**/x.log": "~/app",
	} {
		if got := literalPrefix(glob); got != want {
			t.Errorf("literalPrefix(%q) = %q, want %q", glob, got, want)
		}
	}
}
