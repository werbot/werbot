package shellclass

import (
	"slices"
	"strings"
	"testing"
)

// fuzzMax skips larger inputs: the classifier only sees an SSH channel command.
const fuzzMax = 1 << 16

// fuzzHost is the resource host of the corpora.
const fuzzHost = "db.internal"

// edgeCommands are the ruled edge inputs (Ruling C12): empty, bad UTF-8, NUL,
// 64 KiB, deep nesting, long lists and pipelines.
func edgeCommands() []string {
	return []string{
		"", "\xff\xfe cat x", "cat \x00 /etc/passwd", strings.Repeat("a", fuzzMax),
		strings.Repeat("(", 200) + "ls" + strings.Repeat(")", 200),
		strings.Repeat("ls; ", 500), strings.Repeat("cat x | ", 300) + "cat",
	}
}

func addCommands(f *testing.F) {
	for _, r := range loadCorpus(f) {
		f.Add(r.command)
	}
	for _, c := range edgeCommands() {
		f.Add(c)
	}
}

// FuzzClassify: no panic; a non-empty, ascending list of the seven classes;
// the same result twice; a Normalize name read back from its quoted form is the same name.
func FuzzClassify(f *testing.F) {
	addCommands(f)
	c := mustNew(f)
	order := Classes()
	f.Fuzz(func(t *testing.T, cmd string) {
		if len(cmd) > fuzzMax {
			t.Skip()
		}
		got := c.Classify(cmd, fuzzHost)
		if len(got) == 0 {
			t.Fatalf("Classify(%q) is empty", cmd)
		}
		for i, cl := range got {
			if !Valid(cl) || (i > 0 && slices.Index(order, got[i-1]) >= slices.Index(order, cl)) {
				t.Fatalf("Classify(%q) = %v, want ascending valid classes without repeats", cmd, got)
			}
		}
		if again := c.Classify(cmd, fuzzHost); !slices.Equal(got, again) {
			t.Fatalf("Classify(%q) = %v then %v", cmd, got, again)
		}
		checkNormalize(t, cmd)
	})
}

// fuzzGlobs are valid folder globs; each one gets an only and a deny rule.
var fuzzGlobs = []string{"/**", "~/**", "/etc/*", "/a/**/c"}

func fuzzRules(t *testing.T) []PathRule {
	t.Helper()
	var rules []PathRule
	for _, g := range fuzzGlobs {
		if err := ValidPathGlob(g); err != nil {
			t.Fatalf("ValidPathGlob(%q) = %v", g, err)
		}
		for _, effect := range []string{EffectOnly, EffectDeny} {
			rules = append(rules, PathRule{ID: effect + g, Globs: []string{g}, Access: AccessAny, Effect: effect})
		}
	}
	return rules
}

// FuzzAnalyze: no panic; classes as Classify at cwd "~"; at most MaxPaths
// sorted, distinct, normalized refs; unresolved implies opaque; WithHome and
// both path evaluations agree and do not panic.
func FuzzAnalyze(f *testing.F) {
	addAnalyzeSeeds(f)
	c := mustNew(f)
	f.Fuzz(func(t *testing.T, cmd, cwd string) {
		if len(cmd) > fuzzMax || len(cwd) > fuzzMax {
			t.Skip()
		}
		a := c.Analyze(cmd, fuzzHost, cwd)
		if cwd == "~" && !slices.Equal(a.Classes, c.Classify(cmd, fuzzHost)) {
			t.Fatalf("Analyze(%q).Classes = %v, Classify = %v", cmd, a.Classes, c.Classify(cmd, fuzzHost))
		}
		checkPaths(t, cmd, a)
		_ = a.WithHome("/home/u")
		rules := fuzzRules(t)
		h1, o1 := CompilePaths(rules).Eval(a.Paths, a.Unresolved)
		h2, o2 := EvalPaths(rules, a.Paths, a.Unresolved)
		if !slices.Equal(h1, h2) || !slices.Equal(o1, o2) {
			t.Fatalf("Eval(%q) = %v %v, EvalPaths = %v %v", cmd, h1, o1, h2, o2)
		}
	})
}

func checkPaths(t *testing.T, cmd string, a Analysis) {
	t.Helper()
	if len(a.Paths) > MaxPaths {
		t.Fatalf("Analyze(%q) has %d paths", cmd, len(a.Paths))
	}
	if a.Unresolved && !slices.Contains(a.Classes, Opaque) {
		t.Fatalf("Analyze(%q) is unresolved without opaque: %v", cmd, a.Classes)
	}
	for i, r := range a.Paths {
		if !normalized(r.Path) || (i > 0 && comparePathRefs(a.Paths[i-1], r) >= 0) {
			t.Fatalf("Analyze(%q).Paths = %v, want sorted distinct normalized refs", cmd, a.Paths)
		}
	}
}

func addAnalyzeSeeds(f *testing.F) {
	for _, r := range loadPaths(f) {
		f.Add(r.command, r.cwd)
	}
	for _, r := range loadCorpus(f) {
		f.Add(r.command, "~")
	}
	for _, cwd := range []string{"~", "~/app", "/srv", "/", "rel", "/a/../b"} {
		f.Add("cat x; ls ../y", cwd)
	}
	for _, c := range edgeCommands() {
		f.Add(c, "~")
	}
}

// checkNormalize: a name read back from its quoted form is the same name. A
// quoted name over maxCommand is opaque by design, so it is not read back.
func checkNormalize(t *testing.T, cmd string) {
	t.Helper()
	n := Normalize(cmd)
	if n == "" || len(quote(n)) > maxCommand {
		return
	}
	if back := Normalize(quote(n)); back != n {
		t.Fatalf("Normalize(%q) = %q, Normalize(quote) = %q", cmd, n, back)
	}
}
