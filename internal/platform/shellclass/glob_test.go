package shellclass

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// matchPathCases are glob, path and whether they match.
var matchPathCases = []struct {
	glob, path string
	want       bool
}{
	{"/etc/**", "/etc", true},
	{"/etc/**", "/etc/a/b", true},
	{"/etc/*", "/etc/a/b", false},
	{"/etc/*.conf", "/etc/a.conf", true},
	{"~/app/**", "~/app/x", true},
	{"~/app/**", "/home/u/app/x", false},
	{"/**", "~/x", false},
	{"/**", "/", true},
	{"/a/**/c", "/a/c", true},
	{"/a/**/c", "/a/b/d/c", true},
	{"/", "/", true},
	{"/a", "/a/b", false},
	{"~", "~", true},
	{"~/**", "~", true},
	{"/srv/*", "/srv/.env", true},
}

// Globs match whole segments: "*" inside one segment, "**" any number of segments, "~" only the home symbol.
func TestMatchPath(t *testing.T) {
	for _, tc := range matchPathCases {
		if g := compileGlob(tc.glob); g.match(tc.path) != tc.want {
			t.Errorf("match(%q, %q) = %v, want %v", tc.glob, tc.path, !tc.want, tc.want)
		}
	}
}

// A glob with many "**" segments against a long path is matched in segments × segments steps.
func TestMatchPathManyDoubleStars(t *testing.T) {
	p := "/" + strings.Repeat("a/", 63) + "b"
	start := time.Now()
	if g := compileGlob("/**/**/**/**/x"); g.match(p) {
		t.Fatal("matched a path without the last segment")
	}
	if d := time.Since(start); d > time.Millisecond {
		t.Fatalf("match took %v, want under 1ms", d)
	}
}

// Only absolute or home globs in clean form with whole-segment "**" and a leading "~" are valid.
func TestValidPathGlob(t *testing.T) {
	for _, g := range []string{"/", "~", "~/app/**", "/srv/*/logs/*.log", "/**", "/srv/backup~", "/a/[bc]"} {
		if err := ValidPathGlob(g); err != nil {
			t.Errorf("ValidPathGlob(%q) = %v, want nil", g, err)
		}
	}
	for _, g := range []string{"", "app/**", "/a/../b", "/a/./b", "//a", "/a/", "/a/**b", "~user/x", "/a/~",
		"/a/[", "/a\x00", "/a\u0085b", "/a\xffb", "/" + strings.Repeat("a", 1024), "~/..", "**"} {
		err := ValidPathGlob(g)
		if err == nil {
			t.Errorf("ValidPathGlob(%q) = nil, want an error", g)
			continue
		}
		if !strings.HasPrefix(err.Error(), "invalid path glob: ") {
			t.Errorf("ValidPathGlob(%q) error %q lacks the prefix", g, err)
		}
	}
}

// A path rule needs globs, every one valid, an access of PathAccesses and an effect of PathEffects.
func TestPathRuleValidate(t *testing.T) {
	ok := PathRule{ID: "r", Globs: []string{"~/app/**"}, Access: AccessAny, Effect: EffectOnly}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid rule: %v", err)
	}
	for name, mod := range map[string]func(r *PathRule){
		"no globs":       func(r *PathRule) { r.Globs = nil },
		"relative glob":  func(r *PathRule) { r.Globs = []string{"app/**"} },
		"unknown access": func(r *PathRule) { r.Access = "exec" },
		"unknown effect": func(r *PathRule) { r.Effect = "allow" },
	} {
		r := ok
		mod(&r)
		if r.Validate() == nil {
			t.Errorf("%s: Validate() = nil, want an error", name)
		}
	}
}

func ref(p string, a Access, tree bool) PathRef { return PathRef{Path: p, Access: a, Subtree: tree} }

func evalCase(t *testing.T, rules []PathRule, refs []PathRef, unresolved bool, wantHits, wantOutside string) {
	t.Helper()
	hits, outside := EvalPaths(rules, refs, unresolved)
	if strings.Join(hits, ",") != wantHits || strings.Join(outside, ",") != wantOutside {
		t.Errorf("rules %v refs %v: hits %v outside %v, want %q %q", rules, refs, hits, outside, wantHits, wantOutside)
	}
}

// deny and approval hit on a matching path of a matching access; only reports a path no glob covers.
func TestEvalPaths(t *testing.T) {
	denyEtc := PathRule{ID: "d", Globs: []string{"/etc/**"}, Access: AccessAny, Effect: "deny"}
	denyRead := PathRule{ID: "r", Globs: []string{"/etc/**"}, Access: AccessRead, Effect: "deny"}
	onlyApp := PathRule{ID: "o", Globs: []string{"~/app/**"}, Access: AccessAny, Effect: "only"}
	onlyWrite := PathRule{ID: "w", Globs: []string{"~/app/**"}, Access: AccessWrite, Effect: "only"}
	onlyA := PathRule{ID: "a", Globs: []string{"~/app/**"}, Access: AccessAny, Effect: "only"}
	onlyB := PathRule{ID: "b", Globs: []string{"~/app/logs/**"}, Access: AccessAny, Effect: "only"}
	approve := PathRule{ID: "p", Globs: []string{"/srv/*.key", "/etc/**"}, Access: AccessAny, Effect: "approval"}

	evalCase(t, []PathRule{denyEtc}, []PathRef{ref("/etc/hosts", AccessRead, false)}, false, "d", "")
	evalCase(t, []PathRule{denyRead}, []PathRef{ref("/etc/hosts", AccessWrite, false)}, false, "", "")
	evalCase(t, []PathRule{onlyApp}, []PathRef{ref("~/app/a", AccessRead, false)}, false, "", "")
	evalCase(t, []PathRule{onlyApp}, []PathRef{ref("/tmp/x", AccessWrite, false)}, false, "", "o")
	evalCase(t, []PathRule{onlyWrite}, []PathRef{ref("/etc/hosts", AccessRead, false)}, false, "", "")
	evalCase(t, []PathRule{onlyB, onlyA}, []PathRef{ref("~/app/a", AccessRead, false)}, false, "", "b")
	evalCase(t, []PathRule{approve, denyEtc, denyEtc},
		[]PathRef{ref("/etc/a", AccessRead, false), ref("/etc/b", AccessWrite, false)}, false, "d,p", "")
	evalCase(t, nil, []PathRef{ref("/etc/a", AccessRead, false)}, false, "", "")
}

// A subtree ref hits deny when any path under it can match; only covers it when the glob covers the whole subtree.
func TestEvalPathsSubtree(t *testing.T) {
	rule := func(id, g, effect string) []PathRule {
		return []PathRule{{ID: id, Globs: []string{g}, Access: AccessAny, Effect: effect}}
	}
	evalCase(t, rule("o", "~/app/**", "only"), []PathRef{ref("~/app", AccessWrite, true)}, false, "", "")
	evalCase(t, rule("o", "~/app/*", "only"), []PathRef{ref("~/app", AccessWrite, true)}, false, "", "o")
	evalCase(t, rule("o", "/**", "only"), []PathRef{ref("/", AccessRead, true)}, false, "", "")
	evalCase(t, rule("d", "/etc/**", "deny"), []PathRef{ref("/", AccessRead, true)}, false, "d", "")
	evalCase(t, rule("d", "/srv/app/secret/**", "deny"), []PathRef{ref("/srv", AccessRead, true)}, false, "d", "")
	evalCase(t, rule("d", "/srv/*/secret/**", "deny"), []PathRef{ref("/srv/app", AccessWrite, true)}, false, "d", "")
	evalCase(t, rule("d", "/srv/*.key", "deny"), []PathRef{ref("/srv/app", AccessRead, true)}, false, "", "")
	evalCase(t, rule("d", "/srv/app", "deny"), []PathRef{ref("/srv/app", AccessRead, true)}, false, "d", "")
	evalCase(t, rule("d", "~/x/**", "deny"), []PathRef{ref("/", AccessRead, true)}, false, "", "")
}

// An unresolved analysis puts every only rule outside, whatever its access; deny and approval hit only resolved refs.
func TestEvalPathsUnresolvedOnlyOutside(t *testing.T) {
	rules := []PathRule{
		{ID: "o2", Globs: []string{"/**"}, Access: AccessWrite, Effect: "only"},
		{ID: "o1", Globs: []string{"/**"}, Access: AccessRead, Effect: "only"},
		{ID: "d", Globs: []string{"/**"}, Access: AccessAny, Effect: "deny"},
		{ID: "a", Globs: []string{"/**"}, Access: AccessAny, Effect: "approval"},
	}
	evalCase(t, rules, nil, true, "", "o1,o2")
	evalCase(t, rules[2:], []PathRef{ref("/etc/x", AccessRead, false)}, true, "a,d", "")
}

// ExpandHome rewrites "~" globs from the home folder in a copy; a home that is not an absolute clean path keeps "~".
func TestExpandHome(t *testing.T) {
	rules := []PathRule{{ID: "a", Globs: []string{"~/app/**", "~", "/etc/**"}, Access: AccessAny, Effect: "deny"}}
	got := ExpandHome(rules, "/home/u")
	if want := []string{"/home/u/app/**", "/home/u", "/etc/**"}; !slices.Equal(got[0].Globs, want) {
		t.Errorf("ExpandHome = %v, want %v", got[0].Globs, want)
	}
	if want := []string{"~/app/**", "~", "/etc/**"}; !slices.Equal(rules[0].Globs, want) {
		t.Errorf("source changed to %v", rules[0].Globs)
	}
	if got := ExpandHome(rules, "/"); !slices.Equal(got[0].Globs, []string{"/app/**", "/", "/etc/**"}) {
		t.Errorf("root home: %v", got[0].Globs)
	}
	for _, home := range []string{"", "home/u", "/home/u/", "/home/../u"} {
		if got := ExpandHome(rules, home); !slices.Equal(got[0].Globs, rules[0].Globs) {
			t.Errorf("home %q: %v, want the globs unchanged", home, got[0].Globs)
		}
	}
}
