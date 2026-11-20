package shellclass

import (
	"bufio"
	"math/rand/v2"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

const corpusHost = "db.internal"

type corpusRow struct {
	section, command string
	want             []Class
}

// loadCorpus reads testdata/corpus.txt: "# <section>" opens a section, other "#" lines are comments.
func loadCorpus(t testing.TB) []corpusRow {
	t.Helper()
	f, err := os.Open("testdata/corpus.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var rows []corpusRow
	section := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if name, ok := strings.CutPrefix(line, "# "); ok && !strings.Contains(name, " ") {
			section = name
			continue
		}
		classes, command, ok := strings.Cut(line, "\t")
		if line == "" || strings.HasPrefix(line, "#") || !ok {
			continue
		}
		want := make([]Class, 0, 2)
		for c := range strings.SplitSeq(classes, ",") {
			want = append(want, Class(c))
		}
		slices.Sort(want)
		rows = append(rows, corpusRow{section, command, want})
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return rows
}

func mustNew(t testing.TB, overrides ...Override) *Classifier {
	t.Helper()
	c, err := New(overrides)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func join(cs []Class) string {
	s := make([]string, len(cs))
	for i, c := range cs {
		s[i] = string(c)
	}
	return strings.Join(s, ",")
}

// Every corpus input class is classified as its row says; each section meets its minimum size.
func TestCorpus(t *testing.T) {
	rows := loadCorpus(t)
	if len(rows) < 100 {
		t.Fatalf("corpus has %d rows, want at least 100", len(rows))
	}
	minimum := map[string]int{"read": 15, "write": 15, "destructive": 15, "privilege": 10,
		"secrets": 10, "egress": 10, "opaque": 15, "bypass": 15}
	count := map[string]int{}
	c := mustNew(t)
	for _, r := range rows {
		count[r.section]++
		if got := c.Classify(r.command, corpusHost); join(got) != join(r.want) {
			t.Errorf("[%s] %q: got %s, want %s", r.section, r.command, join(got), join(r.want))
		}
	}
	for s, n := range minimum {
		if count[s] < n {
			t.Errorf("section %q has %d rows, want at least %d", s, count[s], n)
		}
	}
}

// Every spelling of a known argv0 normalizes to the bare name; other paths stay as they are.
func TestNormalize(t *testing.T) {
	for _, in := range []string{`\rm`, "/bin/rm", "/usr/bin/rm", "env rm", "command rm", "busybox rm", "exec rm", "env -i rm", "r''m", `"rm"`} {
		if got := Normalize(in); got != "rm" {
			t.Errorf("Normalize(%q) = %q, want rm", in, got)
		}
	}
	for in, want := range map[string]string{"./deploy.sh": "./deploy.sh", "/opt/x/rm": "/opt/x/rm", "sudo rm": "sudo", "$X": "", "": ""} {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

// Corpus rows and random byte strings never panic and give a sorted, unique, non-empty list of valid classes.
func TestClassifyInvariants(t *testing.T) {
	c := mustNew(t)
	inputs := randomInputs(1000)
	for _, r := range loadCorpus(t) {
		inputs = append(inputs, r.command)
	}
	for _, in := range inputs {
		got := c.Classify(in, corpusHost)
		if len(got) == 0 || !slices.IsSorted(got) || len(slices.Compact(slices.Clone(got))) != len(got) {
			t.Fatalf("%q: got %v, want a sorted unique non-empty list", in, got)
		}
		if i := slices.IndexFunc(got, func(c Class) bool { return !Valid(c) }); i >= 0 {
			t.Fatalf("%q: invalid class %q", in, got[i])
		}
	}
}

// randomInputs returns n fixed pseudo-random strings: mostly shell syntax bytes, some arbitrary bytes.
func randomInputs(n int) []string {
	src := rand.NewChaCha8([32]byte{11})
	alphabet := "abrmf -|&;<>()$`'\"\\{}[]*?=~/.:@\n\t#!0123456789"
	out := make([]string, n)
	for i := range out {
		var size [1]byte
		_, _ = src.Read(size[:])
		b := make([]byte, int(size[0])%64)
		_, _ = src.Read(b)
		for j, c := range b {
			if c%4 != 0 {
				b[j] = alphabet[int(c)%len(alphabet)]
			}
		}
		out[i] = string(b)
	}
	return out
}

func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func nest(prefix string, n int) string {
	cmd := "ls"
	for range n {
		cmd = prefix + quote(cmd)
	}
	return cmd
}

// Empty, oversize, deeply nested, comment-only and assignment-only input classes are classified as [opaque].
func TestClassifyLimits(t *testing.T) {
	c := mustNew(t)
	big := strings.Repeat("ls ", 350000)
	start := time.Now()
	if got := c.Classify(big, ""); join(got) != "opaque" {
		t.Errorf("1 MiB command: got %s, want opaque", join(got))
	}
	if d := time.Since(start); d > time.Millisecond {
		t.Errorf("1 MiB command took %v, want under 1ms", d)
	}
	cases := map[string]string{
		"":                                  "opaque",
		"  \n":                              "opaque",
		strings.Repeat("$(", 10000):         "opaque",
		nest("bash -c ", 9):                 "opaque",
		nest("bash -c ", 8):                 "read",
		strings.Repeat("nohup ", 9) + "ls":  "opaque",
		strings.Repeat("nohup ", 8) + "ls":  "read",
		"# only comment":                    "opaque",
		"X=1":                               "opaque",
		"> /tmp/out":                        "opaque",
		strings.Repeat("a", maxCommand+1):   "opaque",
		"ls " + strings.Repeat("x", 60<<10): "read",
	}
	for in, want := range cases {
		if got := c.Classify(in, ""); join(got) != want {
			t.Errorf("%.40q: got %s, want %s", in, join(got), want)
		}
	}
}

// Overrides match the normalized argv0 plus an argument prefix; the pattern with more fields wins.
func TestOverrides(t *testing.T) {
	c := mustNew(t,
		Override{"./deploy.sh", []Class{Write}},
		Override{"systemctl reload", []Class{Read}},
		Override{"/opt/app/*.sh", []Class{Write}},
		Override{"git", []Class{Read}},
		Override{"git push", []Class{Write}},
	)
	cases := map[string]string{
		"./deploy.sh prod":        "write",
		"systemctl reload nginx":  "read",
		"systemctl restart nginx": "write",
		"/opt/app/run.sh":         "write",
		"/opt/app/sub/run.sh":     "opaque",
		"sudo ./deploy.sh":        "privilege,write",
		"git push":                "write",
		"git log":                 "read",
		"git commit -m x":         "read",
		"cat ~/.ssh/id_rsa | git": "read,secrets",
	}
	for in, want := range cases {
		if got := c.Classify(in, ""); join(got) != want {
			t.Errorf("%q: got %s, want %s", in, join(got), want)
		}
	}
}

// Bad patterns and empty or unknown classes are refused.
func TestNewRejects(t *testing.T) {
	for name, o := range map[string]Override{
		"bad glob":      {"[", []Class{Read}},
		"empty pattern": {"", []Class{Read}},
		"blank pattern": {"  ", []Class{Read}},
		"no classes":    {"ls", nil},
		"unknown class": {"ls", []Class{"bogus"}},
	} {
		if _, err := New([]Override{o}); err == nil {
			t.Errorf("%s: New accepted %+v", name, o)
		}
	}
}

// Network clients are read only for loopback names or the resource host; any other or unknown host is egress.
func TestEgressHost(t *testing.T) {
	c := mustNew(t)
	cases := []struct{ host, cmd, want string }{
		{"10.0.0.5", "curl http://10.0.0.5/x", "read"},
		{"10.0.0.5", "curl http://[::1]:8080/", "read"},
		{"10.0.0.5", "curl LOCALHOST:9000", "read"},
		{"10.0.0.5", "wget -qO- http://203.0.113.5/x", "egress"},
		{"10.0.0.5", "ssh -J jump.example 10.0.0.5", "egress"},
		{"10.0.0.5", "ssh -p 22 admin@10.0.0.5", "read"},
		{"10.0.0.5", "scp 10.0.0.5:/etc/hosts .", "write"},
		{"Db.Internal", "nc -z db.internal 5432", "read"},
		{"", "curl http://10.0.0.5/", "egress"},
		{"", "curl http:///x", "egress"},
		{"", "curl", "egress"},
	}
	for _, tc := range cases {
		if got := c.Classify(tc.cmd, tc.host); join(got) != tc.want {
			t.Errorf("host %q, %q: got %s, want %s", tc.host, tc.cmd, join(got), tc.want)
		}
	}
}
