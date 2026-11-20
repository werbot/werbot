package shellclass

import (
	"slices"
	"testing"
)

func classifyAll(t *testing.T, cases map[string]string) {
	t.Helper()
	c := mustNew(t)
	for in, want := range cases {
		if got := c.Classify(in, corpusHost); join(got) != want {
			t.Errorf("%q: got %s, want %s", in, join(got), want)
		}
	}
}

func requireOpaque(t *testing.T, inputs []string) {
	t.Helper()
	c := mustNew(t)
	for _, in := range inputs {
		if got := c.Classify(in, corpusHost); !slices.Contains(got, Opaque) {
			t.Errorf("%q: got %s, want opaque among the classes", in, join(got))
		}
	}
}

// Input class "assignment of a loader or search-path variable anywhere" is classified as containing opaque (Ruling G1).
func TestClassifyHijackAssignments(t *testing.T) {
	requireOpaque(t, []string{
		"PATH=/tmp ls",
		"PATH=/tmp; ls",
		"LD_PRELOAD=/tmp/x.so ls",
		"export LD_PRELOAD=/tmp/x.so; ls",
		"declare BASH_ENV=/tmp/x; ls",
		"f() { local IFS=/; ls; }",
		"readonly ENV=/tmp/x; ls",
		"typeset LD_LIBRARY_PATH=/tmp; ls",
		"env PATH=/tmp ls",
		"env -i IFS=/ ls",
		"sudo LD_PRELOAD=/tmp/x.so ls",
		": ${PATH:=/tmp}; ls",
	})
	classifyAll(t, map[string]string{
		"LANG=C ls":           "read",
		"env LANG=C ls":       "read",
		"echo ${HOME:-/root}": "read",
	})
}

// Input class "node outside the walker allowlist" is classified as containing opaque (Ruling G2).
func TestClassifyUnknownNodes(t *testing.T) {
	requireOpaque(t, []string{
		"f() { ls; }",
		"coproc ls",
		"(( x = 1 ))",
		"echo $((1 + 2))",
		"let x=1",
		"declare x=1; ls",
		"[[ -f /etc/hosts ]]",
		"for ((i = 0; i < 1; i++)); do ls; done",
		"ls @(a|b)",
		`echo $'\x41'`,
		"a=(x y); ls",
	})
	classifyAll(t, map[string]string{
		"if true; then ls; fi":         "read",
		"while false; do ls; done":     "read",
		"case x in a) ls ;; esac":      "read",
		"{ ls; }":                      "read",
		"(ls)":                         "read",
		"time ls":                      "read",
		"for f in a b; do ls $f; done": "read",
		`echo "$HOME" ${HOME:-/} $(pwd) <(ls) # note`: "read",
		"! ls && ls || ls": "read",
	})
}

// Input class "argv0, subcommand or option word with an expansion, glob or brace" is classified as containing opaque (Ruling G3).
func TestClassifyNonLiteralFlags(t *testing.T) {
	requireOpaque(t, []string{
		"rm -$F /x",
		`rm "-$F" /x`,
		"rm -{r,f} /x",
		"rm -[rf] /x",
		"rm -* /x",
		"systemctl $ACT nginx",
		"git {status,push}",
		`kubectl "$VERB" pods`,
		"r? -rf /",
		"/bin/r* /x",
		"rm $F /x",
		"sed $OPT s/a/b/ f",
		"chmod $MODE /x",
		"find . $EXPR",
		"timeout $T ls",
		"env $VAR ls",
		"bash -c $CMD",
		"su -c $CMD",
		"su $ARGS",
	})
	classifyAll(t, map[string]string{
		"ls /tmp/*.log":                "read",
		`cat "$FILE"`:                  "read",
		"rm -- *.tmp":                  "write",
		`rm -- "$FILE"`:                "write",
		"kubectl logs $POD":            "read",
		`grep -e "$PAT" a.txt`:         "read",
		"curl -H $H http://localhost/": "egress",
	})
}

// Input class "redirection" adds write only for real output files (Ruling G4).
func TestClassifyRedirects(t *testing.T) {
	classifyAll(t, map[string]string{
		"ls > /dev/null":              "read",
		"ls 2>/dev/stderr":            "read",
		"ls >/dev/fd/1 2>/dev/fd/2":   "read",
		"ls 2>&1":                     "read",
		"ls >&2":                      "read",
		"ls <&0":                      "read",
		"ls 3>&-":                     "read",
		"cat <<< x":                   "read",
		"cat <<EOF\nx\nEOF":           "read",
		"ls > out":                    "read,write",
		"ls >> out":                   "read,write",
		"ls >| out":                   "read,write",
		"ls &> out":                   "read,write",
		"ls &>> out":                  "read,write",
		"ls <> out":                   "read,write",
		"ls >& out":                   "read,write",
		"ls > $F":                     "read,write",
		"cat < ~/.ssh/id_rsa":         "read,secrets",
		"ls > /etc/sudoers.d/x":       "privilege,read,write",
		"cat <<EOF\n$(rm -rf /)\nEOF": "destructive,read",
	})
}
