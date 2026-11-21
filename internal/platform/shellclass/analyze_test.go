package shellclass

import (
	"bufio"
	"os"
	"slices"
	"strings"
	"testing"
)

type pathRow struct {
	section, want, cwd, command string
}

// loadPaths reads testdata/paths.txt: "# <section>" opens a section, other "#" lines are comments.
func loadPaths(t testing.TB) []pathRow {
	t.Helper()
	f, err := os.Open("testdata/paths.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var rows []pathRow
	section := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if name, ok := strings.CutPrefix(line, "# "); ok && !strings.Contains(name, " ") {
			section = name
			continue
		}
		fields := strings.SplitN(line, "\t", 3)
		if line == "" || strings.HasPrefix(line, "#") || len(fields) != 3 {
			continue
		}
		rows = append(rows, pathRow{section, fields[0], fields[1], fields[2]})
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return rows
}

// summary renders an analysis as sorted "r:<ref>"/"w:<ref>" tokens plus "?" when unresolved, or "-".
func summary(a Analysis) string {
	var out []string
	for _, r := range a.Paths {
		out = append(out, string(r.Access)[:1]+":"+r.String())
	}
	if a.Unresolved {
		out = append(out, "?")
	}
	if len(out) == 0 {
		return "-"
	}
	slices.Sort(out)
	return strings.Join(out, " ")
}

func sortedTokens(s string) string {
	f := strings.Fields(s)
	slices.Sort(f)
	return strings.Join(f, " ")
}

// Every path corpus input class yields exactly its row's refs and unresolved flag; each section meets its minimum.
func TestPathCorpus(t *testing.T) {
	rows := loadPaths(t)
	if len(rows) < 60 {
		t.Fatalf("path corpus has %d rows, want at least 60", len(rows))
	}
	minimum := map[string]int{"read": 10, "write": 10, "redirect": 5, "cd": 5, "glob": 5, "unresolved": 10, "wrappers": 5}
	count := map[string]int{}
	c := mustNew(t)
	for _, r := range rows {
		count[r.section]++
		a := c.Analyze(r.command, corpusHost, r.cwd)
		if got, want := summary(a), sortedTokens(r.want); got != want {
			t.Errorf("[%s] cwd %s %q: got %s, want %s", r.section, r.cwd, r.command, got, want)
		}
		if a.Unresolved && !slices.Contains(a.Classes, Opaque) {
			t.Errorf("[%s] %q: unresolved without opaque: %v", r.section, r.command, a.Classes)
		}
	}
	for s, n := range minimum {
		if count[s] < n {
			t.Errorf("section %q has %d rows, want at least %d", s, count[s], n)
		}
	}
}

// Classify gives the same classes as Analyze from the home folder for every row of both corpora.
func TestClassifyEqualsAnalyze(t *testing.T) {
	c := mustNew(t)
	var inputs []string
	for _, r := range loadCorpus(t) {
		inputs = append(inputs, r.command)
	}
	for _, r := range loadPaths(t) {
		inputs = append(inputs, r.command)
	}
	for _, in := range inputs {
		if got, want := join(c.Classify(in, corpusHost)), join(c.Analyze(in, corpusHost, "~").Classes); got != want {
			t.Errorf("%q: Classify %s, Analyze %s", in, got, want)
		}
	}
}

// Corpus rows and random strings never panic; refs are normalized, sorted, unique and bounded; unresolved adds opaque.
func TestAnalyzeInvariants(t *testing.T) {
	c := mustNew(t)
	inputs := randomInputs(1000)
	for _, r := range loadCorpus(t) {
		inputs = append(inputs, r.command)
	}
	for _, r := range loadPaths(t) {
		inputs = append(inputs, r.command)
	}
	for _, cwd := range []string{"~", "~/app", "/srv", "", "rel", "~/../x"} {
		for _, in := range inputs {
			checkInvariants(t, in, c.Analyze(in, corpusHost, cwd))
		}
	}
}

func checkInvariants(t *testing.T, in string, a Analysis) {
	t.Helper()
	if len(a.Paths) > MaxPaths || len(a.Classes) == 0 || !slices.IsSorted(a.Classes) {
		t.Fatalf("%q: %d paths, classes %v", in, len(a.Paths), a.Classes)
	}
	if a.Unresolved && !slices.Contains(a.Classes, Opaque) {
		t.Fatalf("%q: unresolved without opaque", in)
	}
	for i, r := range a.Paths {
		if !validRef(r) || (i > 0 && comparePathRefs(a.Paths[i-1], r) >= 0) {
			t.Fatalf("%q: bad, unsorted or repeated ref %+v in %v", in, r, a.Paths)
		}
	}
}

func validRef(r PathRef) bool {
	return normalized(r.Path) && (r.Access == AccessRead || r.Access == AccessWrite) && len(r.String()) <= 1024
}

// normalized reports "/…", "~" or "~/…" without empty, "." or ".." segments.
func normalized(p string) bool {
	if p == "/" || p == "~" {
		return true
	}
	rest, ok := strings.CutPrefix(p, "/")
	if !ok {
		rest, ok = strings.CutPrefix(p, "~/")
	}
	if !ok {
		return false
	}
	for _, s := range strings.Split(rest, "/") {
		if s == "" || s == "." || s == ".." {
			return false
		}
	}
	return true
}

// WithHome resolves "~" refs from the home folder in a copy; refs that become equal merge.
func TestAnalyzeWithHome(t *testing.T) {
	c := mustNew(t)
	a := c.Analyze("cat ~/a /home/u/a ~ /etc/x; rm -r ~/d", corpusHost, "~")
	got := a.WithHome("/home/u")
	if want := "r:/etc/x r:/home/u r:/home/u/a w:/home/u/d/**"; summary(got) != want {
		t.Errorf("WithHome: got %s, want %s", summary(got), want)
	}
	if want := "r:/etc/x r:/home/u/a r:~ r:~/a w:~/d/**"; summary(a) != want {
		t.Errorf("source changed: %s", summary(a))
	}
	root := c.Analyze("cat ~/a ~", corpusHost, "~").WithHome("/")
	if want := "r:/ r:/a"; summary(root) != want {
		t.Errorf("root home: got %s, want %s", summary(root), want)
	}
	if bad := c.Analyze("cat ~/a", corpusHost, "~").WithHome("home"); summary(bad) != "r:~/a" {
		t.Errorf("relative home: got %s, want the ref unchanged", summary(bad))
	}
	long := c.Analyze("cat ~/"+strings.Repeat("a", 1000), corpusHost, "~").WithHome("/" + strings.Repeat("h", 100))
	if !long.Unresolved || len(long.Paths) != 0 || !slices.Contains(long.Classes, Opaque) {
		t.Errorf("over-long expanded path: %s %v", summary(long), long.Classes)
	}
}

// Strings gives the class names and the ref texts; nil for none.
func TestAnalysisStrings(t *testing.T) {
	classes, paths := Analysis{Classes: []Class{Read, Write}, Paths: []PathRef{ref("/srv", AccessWrite, true), ref("~/a", AccessRead, false)}}.Strings()
	if strings.Join(classes, ",") != "read,write" || strings.Join(paths, ",") != "/srv/**,~/a" {
		t.Errorf("Strings() = %v, %v", classes, paths)
	}
	if classes, paths := (Analysis{}).Strings(); classes != nil || paths != nil {
		t.Errorf("empty Strings() = %#v, %#v, want nil", classes, paths)
	}
}
