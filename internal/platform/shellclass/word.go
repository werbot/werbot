package shellclass

import (
	"path"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// marker stands for an expansion inside word text.
const marker = "\x00"

// word is a shell word after quote removal.
type word struct {
	text  string // literal text; marker stands for each expansion
	lit   bool   // no expansions
	split bool   // an unquoted expansion: may become several words
	glob  bool   // an unquoted *, ?, [...] or {a,b} / {a..b}
	tilde tildeForm
}

// plain reports a fully literal word without glob or brace patterns.
func (w word) plain() bool { return w.lit && !w.glob }

func words(ws []*syntax.Word) []word {
	out := make([]word, len(ws))
	for i, w := range ws {
		out[i] = newWord(w)
	}
	return out
}

// wordBuilder keeps the text and, byte for byte, which bytes are active
// (unquoted and unescaped) and so may form glob or brace patterns.
type wordBuilder struct {
	text, active []byte
	w            word
}

func newWord(sw *syntax.Word) word {
	b := wordBuilder{w: word{lit: true}}
	for _, p := range sw.Parts {
		b.part(p)
	}
	b.w.text = string(b.text)
	b.w.glob = isGlob(b.text, b.active)
	b.w.tilde = tildeOf(b.text, b.active)
	return b.w
}

// valueWord is an option value cut from a longer word ("--name=value",
// "-ovalue", "of=value"). Its quoting is unknown, so it is taken at its
// worst: any pattern byte is a glob and a leading "~" is unresolved.
func valueWord(s string) word {
	w := word{text: s, lit: !strings.Contains(s, marker), glob: strings.ContainsAny(s, "*?[{")}
	if strings.HasPrefix(s, "~") {
		w.tilde = tildeBad
	}
	return w
}

func (b *wordBuilder) add(s string, active bool) {
	for i := range len(s) {
		b.text = append(b.text, s[i])
		if active {
			b.active = append(b.active, s[i])
		} else {
			b.active = append(b.active, ' ')
		}
	}
}

func (b *wordBuilder) expansion(split bool) {
	b.add(marker, false)
	b.w.lit = false
	b.w.split = b.w.split || split
}

func (b *wordBuilder) part(p syntax.WordPart) {
	switch p := p.(type) {
	case *syntax.Lit:
		b.unquoted(p.Value)
	case *syntax.SglQuoted:
		if p.Dollar {
			b.expansion(false)
			return
		}
		b.add(p.Value, false)
	case *syntax.DblQuoted:
		b.quoted(p)
	default:
		b.expansion(true)
	}
}

// unquoted removes backslash escapes; an escaped byte is not active.
func (b *wordBuilder) unquoted(s string) {
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 == len(s) {
			b.add(s[i:i+1], true)
			continue
		}
		i++
		if s[i] != '\n' {
			b.add(s[i:i+1], false)
		}
	}
}

func (b *wordBuilder) quoted(q *syntax.DblQuoted) {
	if q.Dollar {
		b.expansion(false)
		return
	}
	for _, p := range q.Parts {
		lit, ok := p.(*syntax.Lit)
		if !ok {
			b.expansion(false)
			continue
		}
		b.add(dquoteUnescape(lit.Value), false)
	}
}

// dquoteUnescape removes the backslashes that double quotes honour.
func dquoteUnescape(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && strings.IndexByte("$`\"\\\n", s[i+1]) >= 0 {
			i++
			if s[i] == '\n' {
				continue
			}
		}
		out.WriteByte(s[i])
	}
	return out.String()
}

// isGlob reports an active *, ?, a [ closed later, or a brace pattern.
func isGlob(text, active []byte) bool {
	for i, c := range active {
		switch c {
		case '*', '?':
			return true
		case '[':
			if slices.Contains(text[i+1:], ']') {
				return true
			}
		case '{':
			if brace(text[i+1:], active[i+1:]) {
				return true
			}
		}
	}
	return false
}

// brace reports an active "}" with "," or ".." before it.
func brace(text, active []byte) bool {
	end := slices.Index(active, '}')
	if end < 0 {
		return false
	}
	inner := string(text[:end])
	return strings.Contains(inner, ",") || strings.Contains(inner, "..")
}

// systemDirs hold binaries that are named by their base name.
var systemDirs = []string{"/bin", "/sbin", "/usr/bin", "/usr/sbin", "/usr/local/bin", "/usr/local/sbin"}

func normalizeName(s string) string {
	if strings.HasPrefix(s, "/") && slices.Contains(systemDirs, path.Dir(s)) {
		return path.Base(s)
	}
	return s
}

// Normalize returns the command name of a command line: escapes and quotes
// removed, leading wrappers (env, command, busybox, exec, nohup, …) dropped,
// and a binary in a system directory reduced to its base name. Other paths
// are returned as they are; "" means the name is not literal.
func Normalize(argv0 string) string {
	f, ok := parse(argv0, 0)
	if !ok {
		return ""
	}
	for n := range syntax.Preorder(f) {
		if call, ok := n.(*syntax.CallExpr); ok && len(call.Args) > 0 {
			return unwrapName(words(call.Args))
		}
	}
	return ""
}

func unwrapName(args []word) string {
	for range maxDepth + 1 {
		if !args[0].plain() {
			return ""
		}
		name := normalizeName(args[0].text)
		w, ok := wrappers[name]
		if !ok || w.adds != 0 {
			return name
		}
		rest, _, done := w.unwrap(args[1:])
		if done {
			return name
		}
		args = rest
	}
	return ""
}

// secretParts are path components that name credential files.
var secretParts = []string{".ssh", ".env", ".git-credentials", ".netrc", ".pgpass"}

// secretPaths are path fragments that name credential files.
var secretPaths = []string{"/etc/shadow", "/etc/gshadow", ".aws/credentials", ".kube/config"}

// pathClasses returns secrets when s names a credential file.
func pathClasses(s string) set {
	s = path.Clean(s)
	for _, p := range secretPaths {
		if strings.Contains(s, p) {
			return of(Secrets)
		}
	}
	if strings.Contains(s, "/proc/") && strings.HasSuffix(s, "/environ") {
		return of(Secrets)
	}
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == '/' || r == '=' }) {
		if slices.Contains(secretParts, part) || strings.HasPrefix(part, ".env.") {
			return of(Secrets)
		}
	}
	return 0
}

func argClasses(args []word) set {
	var s set
	for _, a := range args[1:] {
		s |= pathClasses(a.text)
	}
	return s
}

// sudoers returns privilege when s names the sudo configuration.
func sudoers(s string) set {
	if strings.Contains(s, "/etc/sudoers") {
		return of(Privilege)
	}
	return 0
}
