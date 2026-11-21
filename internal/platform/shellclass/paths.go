package shellclass

import (
	"cmp"
	"path"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Access is the kind of access a command has to a path.
type Access string

// The accesses; AccessAny appears only in a PathRule.
const (
	AccessRead  Access = "read"
	AccessWrite Access = "write"
	AccessAny   Access = "any"
)

// PathRef is one path a command touches. Path is normalized: "/…", "~" or
// "~/…" without ".", ".." or empty segments. Subtree means the command may
// touch anything under Path.
type PathRef struct {
	Path    string
	Access  Access
	Subtree bool
}

// String returns Path, or Path + "/**" for a subtree ("/" gives "/**").
func (r PathRef) String() string {
	if !r.Subtree {
		return r.Path
	}
	return strings.TrimSuffix(r.Path, "/") + "/**"
}

// Analysis is the result of Analyze. Paths are sorted by (Path, Access,
// Subtree) without repeats. Unresolved means a path of the command could not
// be computed; such a path is not in Paths and Classes contains opaque.
type Analysis struct {
	Classes    []Class
	Paths      []PathRef
	Unresolved bool
}

// Strings returns the class names and the PathRef texts of a; nil for none.
func (a Analysis) Strings() (classes, paths []string) {
	for _, c := range a.Classes {
		classes = append(classes, string(c))
	}
	for _, r := range a.Paths {
		paths = append(paths, r.String())
	}
	return classes, paths
}

// MaxPaths bounds the refs of one analysis; more refs make it unresolved.
const MaxPaths = 256

const (
	maxPathLen  = 1024 // a longer ref string is unresolved
	maxSegments = 2048 // path segments over all refs of one analysis
)

// WithHome returns a copy with "~" and "~/x" refs resolved from home ("/"
// gives "/x"). A home that is not an absolute clean path leaves the refs as
// they are.
func (a Analysis) WithHome(home string) Analysis {
	out := Analysis{Classes: slices.Clone(a.Classes), Unresolved: a.Unresolved}
	var refs refSet
	for _, r := range a.Paths {
		r.Path = expandTilde(r.Path, home)
		if !refs.add(r) {
			out.Unresolved = true
		}
	}
	out.Paths = refs.sorted()
	if out.Unresolved && !slices.Contains(out.Classes, Opaque) {
		out.Classes = append(out.Classes, Opaque)
		slices.Sort(out.Classes)
	}
	return out
}

func homeOK(home string) bool {
	return strings.HasPrefix(home, "/") && path.Clean(home) == home && clean(home)
}

// expandTilde resolves a leading "~" segment of p from home.
func expandTilde(p, home string) string {
	if !homeOK(home) {
		return p
	}
	if p == "~" {
		return home
	}
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		return path.Join(home, rest)
	}
	return p
}

// refSet collects refs without repeats within the bounds.
type refSet struct {
	seen map[PathRef]bool
	list []PathRef
	segs int
}

// add records r; it reports false when r breaks a bound and is dropped.
func (s *refSet) add(r PathRef) bool {
	if s.seen[r] {
		return true
	}
	n := strings.Count(r.Path, "/") + 1
	if len(r.String()) > maxPathLen || len(s.list) == MaxPaths || s.segs+n > maxSegments {
		return false
	}
	if s.seen == nil {
		s.seen = map[PathRef]bool{}
	}
	s.seen[r] = true
	s.list = append(s.list, r)
	s.segs += n
	return true
}

func (s *refSet) sorted() []PathRef {
	slices.SortFunc(s.list, comparePathRefs)
	return s.list
}

func comparePathRefs(a, b PathRef) int {
	return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Access, b.Access), cmp.Compare(boolInt(a.Subtree), boolInt(b.Subtree)))
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// tildeForm tells how a word starts with "~" (Ruling G2).
type tildeForm uint8

const (
	tildeNone tildeForm = iota // no active "~" first: a literal name
	tildeHome                  // an active "~" alone or before an active "/"
	tildeBad                   // ~user, ~+, or quoted or escaped bytes in the prefix
)

// tildeOf classifies the first byte of a word from its text and active bytes.
func tildeOf(text, active []byte) tildeForm {
	switch {
	case len(active) == 0 || active[0] != '~':
		return tildeNone
	case len(text) == 1 || active[1] == '/':
		return tildeHome
	}
	return tildeBad
}

// clean reports text without control characters or invalid UTF-8 (Ruling G7).
func clean(s string) bool {
	return utf8.ValidString(s) && !strings.ContainsFunc(s, unicode.IsControl)
}

// pathLike reports a word that names a path by its form (Ruling G4).
func pathLike(s string) bool {
	return strings.HasPrefix(s, "/") || strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") ||
		strings.HasPrefix(s, "~")
}

// streams are paths that name the standard streams, not files.
var streams = []string{"/dev/null", "/dev/stdin", "/dev/stdout", "/dev/stderr"}

// globDir cuts a glob word to the segments before the first one with a
// pattern byte; "" stands for the cwd.
func globDir(s string) string {
	segs := strings.Split(s, "/")
	i := slices.IndexFunc(segs, func(seg string) bool { return strings.ContainsAny(seg, "*?[{") })
	dir := strings.Join(segs[:i], "/")
	if dir == "" && strings.HasPrefix(s, "/") {
		return "/"
	}
	return dir
}

// scope is the state a command runs in.
type scope struct {
	cwd     string // normalized; "" when unknown
	noTilde bool   // "~" paths are unresolved
	noRel   bool   // relative paths are unresolved
	priv    bool   // a privilege wrapper runs the command
	depth   int
}

// resolve normalizes a path word's text from the scope; ok is false when the
// path cannot be computed.
func (sc scope) resolve(text string, home bool) (string, bool) {
	switch {
	case home && sc.noTilde:
		return "", false
	case home:
		return joinPath("~", text[1:])
	case strings.HasPrefix(text, "/"):
		return path.Clean(text), true
	case sc.noRel || sc.cwd == "":
		return "", false
	}
	return joinPath(sc.cwd, text)
}

// joinPath resolves rel from base ("/…", "~" or "~/…"); a result above "~" fails.
// Under "~" a leading "/" of rel is ignored.
func joinPath(base, rel string) (string, bool) {
	if strings.HasPrefix(base, "/") {
		return path.Join(base, rel), true
	}
	x := path.Join(".", base[1:], rel)
	switch {
	case x == ".." || strings.HasPrefix(x, "../"):
		return "", false
	case x == ".":
		return "~", true
	}
	return "~/" + x, true
}

// startCwd normalizes the cwd given to Analyze; "" means unknown.
func startCwd(cwd string) string {
	if !clean(cwd) {
		return ""
	}
	switch {
	case strings.HasPrefix(cwd, "/"):
		return path.Clean(cwd)
	case cwd == "~" || strings.HasPrefix(cwd, "~/"):
		p, _ := joinPath("~", cwd[1:])
		return p
	}
	return ""
}
