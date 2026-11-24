package parity

import (
	"io/fs"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var (
	testName = regexp.MustCompile(`Test[A-Za-z0-9_]+`)
	tsFile   = regexp.MustCompile(`web/[\w./-]+\.(?:spec|redteam)\.ts`)
	docLink  = regexp.MustCompile(`\]\(\.\./([\w-]+\.md)(?:#([\w-]+))?\)`)
	nonSlug  = regexp.MustCompile(`[^a-z0-9 _-]`)
)

// rows returns the cells of each table row under the "## <section>" heading.
func rows(t *testing.T, doc, section string) [][]string {
	t.Helper()
	var out [][]string
	in := false
	for _, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(line, "## ") {
			in = strings.TrimPrefix(line, "## ") == section
			continue
		}
		if !in || !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "| "), " | ")
		if strings.HasPrefix(cells[0], "---") || cells[0] == "#" || cells[0] == "Failure" {
			continue
		}
		out = append(out, cells)
	}
	return out
}

// repo opens the repository root; reads cannot leave it.
func repo(t *testing.T) *os.Root {
	t.Helper()
	r, err := os.OpenRoot("../..")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

// testSources joins every *_test.go file of the repository (build tags ignored).
func testSources(t *testing.T, r *os.Root) string {
	t.Helper()
	var sb strings.Builder
	err := fs.WalkDir(r.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "node_modules" || d.Name() == ".git" || d.Name() == ".superpowers") {
			return fs.SkipDir
		}
		if strings.HasSuffix(p, "_test.go") {
			b, err := fs.ReadFile(r.FS(), p)
			if err != nil {
				return err
			}
			sb.Write(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return sb.String()
}

func slugs(t *testing.T, r *os.Root, page string) map[string]bool {
	t.Helper()
	b, err := fs.ReadFile(r.FS(), path.Join("docs", page))
	if err != nil {
		t.Fatalf("linked page docs/%s: %v", page, err)
	}
	out := map[string]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		if h := strings.TrimLeft(line, "#"); h != line && strings.HasPrefix(h, " ") {
			s := nonSlug.ReplaceAllString(strings.ToLower(strings.TrimSpace(h)), "")
			out[strings.ReplaceAll(s, " ", "-")] = true
		}
	}
	return out
}

func checkParityRows(t *testing.T, doc string) {
	parity := rows(t, doc, "Parity")
	if len(parity) != 25 {
		t.Fatalf("parity table has %d rows, want 25", len(parity))
	}
	for i, r := range parity {
		if n, err := strconv.Atoi(r[0]); err != nil || n != i+1 {
			t.Errorf("parity row %d is numbered %q", i+1, r[0])
		}
		if len(r) != 3 || strings.TrimSpace(r[2]) == "" {
			t.Errorf("parity row %d has no proof", i+1)
		}
	}
}

func checkFailureRows(t *testing.T, doc string) {
	failures := rows(t, doc, "Failure matrix")
	if len(failures) != 9 {
		t.Fatalf("failure matrix has %d rows, want 9", len(failures))
	}
	for i, r := range failures {
		if len(r) != 2 || !testName.MatchString(r[1]) && !tsFile.MatchString(r[1]) {
			t.Errorf("failure row %d has no test", i+1)
		}
	}
}

func TestParityTableReferencesRealTests(t *testing.T) {
	r := repo(t)
	b, err := fs.ReadFile(r.FS(), "docs/maintainers/parity.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	checkParityRows(t, doc)
	checkFailureRows(t, doc)

	src := testSources(t, r)
	for _, name := range testName.FindAllString(doc, -1) {
		if !strings.Contains(src, "func "+name+"(") {
			t.Errorf("test %s not found", name)
		}
	}
	for _, f := range tsFile.FindAllString(doc, -1) {
		if _, err := r.Stat(f); err != nil {
			t.Errorf("file %s not found", f)
		}
	}
	for _, m := range docLink.FindAllStringSubmatch(doc, -1) {
		if m[2] != "" && !slugs(t, r, m[1])[m[2]] {
			t.Errorf("anchor %s#%s not found", m[1], m[2])
		}
	}
}
