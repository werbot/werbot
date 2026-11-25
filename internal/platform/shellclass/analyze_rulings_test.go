package shellclass

import (
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func analyzeAll(t *testing.T, cwd string, cases map[string]string) {
	t.Helper()
	c := mustNew(t)
	for in, want := range cases {
		if got := summary(c.Analyze(in, corpusHost, cwd)); got != sortedTokens(want) {
			t.Errorf("cwd %q, %q: got %s, want %s", cwd, in, got, want)
		}
	}
}

// Input classes "path in a variable, command substitution, ~user, beyond ~, non-literal cd, xargs,
// control character, invalid UTF-8, unparsable" are unresolved and add opaque (Rulings G2, G7).
func TestAnalyzeUnresolvedAddsOpaque(t *testing.T) {
	c := mustNew(t)
	for _, in := range []string{
		"cat $F", "cat $(echo x)", "cat `echo x`", "cat ~bob/x", "cat ../x", "cat ~/../../x", "cd $D && ls",
		"echo a | xargs cat", "cat 'a\x01b'", "cat '/etc/\u0085x'", "cat '/etc/\xff'", "cat /etc/x |", "",
		"f() { cat /etc/x; }", "export X=1; cat x",
	} {
		a := c.Analyze(in, corpusHost, "~")
		if !a.Unresolved || !slices.Contains(a.Classes, Opaque) {
			t.Errorf("%q: unresolved %v, classes %v; want unresolved and opaque", in, a.Unresolved, a.Classes)
		}
	}
	if a := c.Analyze("cat /etc/x", corpusHost, "~"); a.Unresolved || slices.Contains(a.Classes, Opaque) {
		t.Errorf("literal path: unresolved %v, classes %v", a.Unresolved, a.Classes)
	}
}

// Input class ".", "..", "//" in absolute and relative paths from "~", "~/app" and "/srv" gives clean refs.
func TestAnalyzeNormalizesPaths(t *testing.T) {
	cmd := "cat /etc/./a//b/../c x/../y ./z .. ."
	analyzeAll(t, "~", map[string]string{cmd: "? r:/etc/a/c r:~/y r:~/z r:~"})
	analyzeAll(t, "~/app", map[string]string{cmd: "r:/etc/a/c r:~/app/y r:~/app/z r:~ r:~/app"})
	analyzeAll(t, "/srv", map[string]string{cmd: "r:/etc/a/c r:/srv/y r:/srv/z r:/ r:/srv"})
	analyzeAll(t, "/srv/../x/", map[string]string{"cat a": "r:/x/a"})
	analyzeAll(t, "~/app/..", map[string]string{"cat a": "r:~/a"})
	for _, bad := range []string{"", "app", "~bob", "~/..", "~/../x", "-"} {
		analyzeAll(t, bad, map[string]string{"cat a": "?", "cat /a": "r:/a"})
	}
}

// Input class "redirection or operand naming a standard stream or a descriptor duplication" yields no path.
func TestAnalyzeStreamsIgnored(t *testing.T) {
	analyzeAll(t, "~", map[string]string{
		"echo x > /dev/null":                 "-",
		"echo x 2>/dev/null":                 "-",
		"echo x < /dev/stdin":                "-",
		"echo x > /dev/stderr":               "-",
		"echo x >/dev/stdout 2>&1 >&2 3>&-":  "-",
		"cat /dev/null":                      "-",
		"cat <<EOF\n/etc/x\nEOF":             "-",
		"cat <<< /etc/x":                     "-",
		"cat <<EOF\n$(cat /etc/x)\nEOF":      "r:/etc/x",
		"echo x > /dev/null/../../etc/hosts": "w:/etc/hosts",
	})
}

// Input class "more refs than MaxPaths or more segments than the budget" keeps the first refs and is unresolved.
func TestAnalyzeTooManyPaths(t *testing.T) {
	c := mustNew(t)
	var b strings.Builder
	b.WriteString("touch")
	for i := range 300 {
		b.WriteString(" /f" + strconv.Itoa(i))
	}
	a := c.Analyze(b.String(), corpusHost, "~")
	if len(a.Paths) != MaxPaths || !a.Unresolved || !slices.Contains(a.Classes, Opaque) {
		t.Errorf("300 files: %d paths, unresolved %v, classes %v", len(a.Paths), a.Unresolved, a.Classes)
	}
	deep := "cat"
	for i := range 25 {
		deep += " /" + strings.Repeat("a/", 99) + strconv.Itoa(i)
	}
	if a := c.Analyze(deep, corpusHost, "~"); !a.Unresolved || len(a.Paths) == 0 {
		t.Errorf("segment budget: %d paths, unresolved %v", len(a.Paths), a.Unresolved)
	}
	long := "cat /" + strings.Repeat("a", 1024)
	if a := c.Analyze(long, corpusHost, "~"); !a.Unresolved || len(a.Paths) != 0 {
		t.Errorf("path over 1024 bytes: %s", summary(a))
	}
}

// Input class "tilde prefix": only an active "~" up to an active "/" is home; quoted or escaped forms per Ruling G2.
func TestAnalyzeTildeForms(t *testing.T) {
	analyzeAll(t, "/srv", map[string]string{
		"cat ~":          "r:~",
		"cat ~/x":        "r:~/x",
		"cat ~/":         "r:~",
		`cat ~"/x"`:      "?",
		`cat ~\/x`:       "?",
		"cat ~bob/x":     "?",
		"cat ~+/x":       "?",
		`cat \~/x`:       "r:/srv/~/x",
		`cat "~/x"`:      "r:/srv/~/x",
		"cat x/~":        "r:/srv/x/~",
		"cat a=~/x":      "?",
		"dd if=~/x":      "?",
		"sort -o=~/x a":  "? r:/srv/a",
		"echo --out=~/x": "?",
		"cat ~/x*":       "r:~/**",
	})
}

// Input classes "wrapper or command that changes the cwd or HOME" make relative or "~" paths unresolved (Ruling G3).
func TestAnalyzeCwdChangers(t *testing.T) {
	analyzeAll(t, "~", map[string]string{
		"env -C /srv cat x":         "?",
		"env --chdir=/srv cat x":    "?",
		"sudo -D /srv cat x":        "?",
		"sudo --chdir /srv cat x":   "?",
		"sudo -i cat x":             "?",
		"sudo --login cat x":        "?",
		"sudo -u bob cat x":         "r:~/x",
		"pkexec cat x":              "?",
		"su - -c 'cat x'":           "?",
		"su -l -c 'cat x'":          "?",
		"su --login -c 'cat /x'":    "r:/x",
		"su -c 'cat x'":             "r:~/x",
		"su -c 'cat ~/x'":           "?",
		"doas sh -c 'cat ~/x'":      "?",
		"pkexec bash -c 'cat ~'":    "?",
		"sudo -- sh -c 'cat /x'":    "r:/x",
		"pushd /srv; cat /x":        "r:/x",
		"popd && cat x":             "?",
		"cd srv && cat x":           "?",
		"cd . && cat x":             "r:~/x",
		"HOME=/x; cat ~/a":          "?",
		"cat ~/a; HOME=/x":          "?",
		"export HOME=/x; cat /a":    "? r:/a",
		"env CDPATH=/x cat a":       "?",
		"sudo HOME=/x cat /a":       "r:/a",
		"echo ${HOME:=/x}; cat ~/a": "?",
		"bash -c 'HOME=/x'; cat /a": "r:/a",
		"read HOME; cat ~/a":        "?",
		"nohup cd /srv && cat x":    "r:~/x",
		"command cd /srv && cat x":  "r:/srv/x",
		"env -C /srv ls; cat x":     "? r:~/x",
		"eval cd /srv && cat x":     "? r:/srv",
		". ./env.sh && cat x":       "? r:~/env.sh",
	})
}

// Input classes "argv0 with a slash, path-like literal of any command, --opt=<path>" are read refs (Ruling G4).
func TestAnalyzeArgvAndOptionPaths(t *testing.T) {
	analyzeAll(t, "~", map[string]string{
		"./run.sh":                         "r:~/run.sh",
		"../bin/x":                         "?",
		"/opt/app/bin/tool -v":             "r:/opt/app/bin/tool",
		"/usr/bin/cat /etc/x":              "r:/usr/bin/cat r:/etc/x",
		"unknowncmd /etc/x ./y ~/z name":   "r:/etc/x r:~/y r:~/z",
		"python3 --config=/etc/app.yml":    "r:/etc/app.yml",
		"kubectl logs pod --since=1h":      "-",
		"sort --output=/tmp/s data":        "r:~/data w:/tmp/s",
		"sort --out=/tmp/s data":           "r:~/data w:/tmp/s",
		"grep --exclude-from=/etc/x p f":   "r:/etc/x r:~/f",
		"grep --file=/etc/pats f":          "r:/etc/pats r:~/f",
		"tar -cf /tmp/a.tar -C /srv app":   "r:/srv/app/** w:/tmp/a.tar",
		"tar -x -f a.tar":                  "r:~/a.tar w:~/**",
		"tar -T /etc/list -cf - .":         "r:/etc/list r:~/**",
		"tar --frobnicate a":               "?",
		"unzip -d /srv/x a.zip":            "r:~/a.zip w:/srv/x/**",
		"scp a b c:/x":                     "r:~/a r:~/b",
		"scp -r a dst":                     "r:~/a/** w:~/dst/**",
		"rsync -a src/ /backup":            "r:~/src/** w:/backup/**",
		"find / -fprint /tmp/out -name x":  "r:/** w:/tmp/out",
		"find . -delete":                   "w:~/**",
		"find -L /srv -execdir cat a \\;":  "? r:/srv/**",
		"find /srv -exec cat /etc/x \\;":   "r:/srv/** r:/etc/x",
		"chmod -Rv 755 /srv":               "w:/srv/**",
		"chmod -- 644 /srv/a":              "w:/srv/a",
		"chown --reference=/etc/r /srv/a":  "r:/etc/r w:/srv/a",
		"ln -s /etc/x":                     "r:/etc/x w:~/**",
		"git-receive-pack /srv/repo.git":   "w:/srv/repo.git",
		"bash script.sh /etc/x":            "r:/etc/x",
		"sed -e s/a/b/ -i /srv/a /srv/b":   "w:/srv/a w:/srv/b",
		"jq -f /etc/q.jq --arg k v data":   "r:/etc/q.jq r:~/data r:~/k r:~/v",
		"grep -rl --include=*.go func src": "r:~/src/**",
	})
}

// Input class "copy, move, install or link into a folder operand" hits a deny write glob on a name under that folder.
func TestAnalyzeDirectoryTargetHitsDeny(t *testing.T) {
	c := mustNew(t)
	rules := []PathRule{{ID: "k", Globs: []string{"/srv/*.key"}, Access: AccessWrite, Effect: "deny"}}
	for _, in := range []string{"mv x.key /srv", "cp x.key /srv", "install x.key /srv", "ln -s x.key /srv",
		"cp -t /srv x.key", "scp x.key /srv", "rsync x.key /srv", "cd /srv && ln -s /tmp/x.key"} {
		a := c.Analyze(in, corpusHost, "~")
		if hits, _ := EvalPaths(rules, a.Paths, a.Unresolved); len(hits) != 1 {
			t.Errorf("%q: refs %s, want a hit of the deny write rule", in, summary(a))
		}
	}
}

// Input class "move a folder": a deny write glob on a name below the moved folder hits.
func TestAnalyzeMovedFolderHitsDeny(t *testing.T) {
	c := mustNew(t)
	rules := []PathRule{{ID: "d", Globs: []string{"/srv/*/config/*.yml"}, Access: AccessWrite, Effect: "deny"}}
	for _, in := range []string{"mv /srv/app /tmp/x", "mv /srv/app/config /tmp/c", "mv -t /tmp /srv/app"} {
		a := c.Analyze(in, corpusHost, "~")
		if hits, _ := EvalPaths(rules, a.Paths, a.Unresolved); len(hits) != 1 {
			t.Errorf("%q: refs %s, want a hit of the deny write rule", in, summary(a))
		}
	}
}

// Input class "clauses nested 40 deep, with and without a cd inside": the walk stays linear in the
// depth (no pass per branch combination) and keeps the absolute path of every level.
func TestAnalyzeNestedClausesBounded(t *testing.T) {
	c := mustNew(t)
	for _, inner := range []string{"true", "cd /x"} {
		in := inner
		for range 40 {
			in = "while " + in + "; do rm /srv/a; done"
		}
		done := make(chan Analysis, 1)
		go func() { done <- c.Analyze(in, corpusHost, "~") }()
		select {
		case a := <-done:
			if got := summary(a); got != "w:/srv/a" {
				t.Errorf("inner %q: got %s, want w:/srv/a", inner, got)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("inner %q: analysis of 40 nested clauses did not finish in 5 s", inner)
		}
	}
}
