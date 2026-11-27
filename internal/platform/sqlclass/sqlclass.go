// Package sqlclass sorts a query text into the risk classes of shellclass
// and lists the tables it reads and writes. Each dialect registers its
// Parser; Classify wraps it with fail-closed bounds: any failure is opaque.
package sqlclass

import (
	"fmt"
	"path"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/werbot/werbot/internal/platform/shellclass"
)

// Dialect names a query language; it equals the connector type name.
type Dialect string

// Dialects.
const (
	Postgres Dialect = "postgres"
	MySQL    Dialect = "mysql"
)

// MaxText is the longest text Classify passes to a parser.
const MaxText = 1 << 20

const (
	maxGlobs   = 64
	maxGlobLen = 255
)

// Options tune one classification.
type Options struct {
	SecretTables []string // globs over "schema.table" (path.Match)
}

// Result is the classification of one text. Classes are ascending and never
// empty; Tables and WriteTables are ascending without duplicates. Callers
// must not modify the slices (results are cached and shared).
type Result struct {
	Classes             []shellclass.Class
	Tables, WriteTables []string
	Statements          int
	// Transient marks an opaque result that may change on retry (parser busy,
	// not ready or broken, panic, text too long, no parser). It is never cached.
	Transient bool
}

// Parser classifies texts of one dialect. It is safe for concurrent use.
// A parser reports a busy, not-ready or broken state as a Transient result;
// Classify turns that into a transient [opaque].
type Parser interface {
	Classify(sql string, opt Options) Result
}

// ProtectedParams are the session settings whose change is privilege. The
// lexing settings keep the server's statement boundaries equal to the parser's.
var ProtectedParams = []string{
	"statement_timeout", "default_transaction_read_only", "transaction_read_only",
	"role", "session_authorization", "search_path",
	"standard_conforming_strings", "client_encoding", "backslash_quote",
	// mysql
	"max_execution_time", "sql_safe_updates", "transaction_isolation",
}

// parsers is filled by Register from package init functions only, so reads
// need no lock.
var parsers = map[Dialect]Parser{}

// Register installs the parser of d. A second parser for d panics.
func Register(d Dialect, p Parser) {
	if _, ok := parsers[d]; ok {
		panic(fmt.Sprintf("sqlclass: dialect %q registered twice", d))
	}
	parsers[d] = p
}

// Classify classifies sql with the parser of d, then marks secret tables.
// A missing parser, a text over MaxText, a panic or a transient parser result
// give a transient [opaque]; a result without classes gives [opaque].
func Classify(d Dialect, sql string, opt Options) (r Result) {
	p, ok := parsers[d]
	if !ok || len(sql) > MaxText {
		return transient()
	}
	defer func() {
		if recover() != nil {
			r = transient()
		}
	}()
	r = p.Classify(sql, opt)
	switch {
	case r.Transient:
		return transient()
	case len(r.Classes) == 0:
		return Opaque()
	}
	return MarkSecrets(r, opt.SecretTables)
}

// Opaque is the deterministic fail-closed result.
func Opaque() Result {
	return Result{Classes: []shellclass.Class{shellclass.Opaque}}
}

func transient() Result {
	r := Opaque()
	r.Transient = true
	return r
}

// MarkSecrets adds secrets when a table of r matches one of globs. It returns
// a copy with new Classes and leaves r unchanged.
func MarkSecrets(r Result, globs []string) Result {
	if len(globs) == 0 || slices.Contains(r.Classes, shellclass.Secrets) {
		return r
	}
	if matchAny(r.Tables, globs) || matchAny(r.WriteTables, globs) {
		r.Classes = Merge(r.Classes, []shellclass.Class{shellclass.Secrets})
	}
	return r
}

// matchAny reports whether a table matches a glob; bad globs never match
// (ValidGlobs rejects them on save).
func matchAny(tables, globs []string) bool {
	for _, t := range tables {
		for _, g := range globs {
			if ok, _ := path.Match(g, t); ok {
				return true
			}
		}
	}
	return false
}

// Protected reports whether name is in ProtectedParams, ignoring case.
func Protected(name string) bool {
	return slices.ContainsFunc(ProtectedParams, func(p string) bool { return strings.EqualFold(p, name) })
}

// Merge returns the ascending union of a and b in a new slice.
func Merge(a, b []shellclass.Class) []shellclass.Class {
	out := slices.Concat(a, b)
	slices.Sort(out)
	return slices.Compact(out)
}

// ValidGlobs checks secret table globs: at most 64, each at most 255
// characters and a valid path.Match pattern.
func ValidGlobs(globs []string) error {
	if len(globs) > maxGlobs {
		return fmt.Errorf("sqlclass: more than %d secret table globs", maxGlobs)
	}
	for _, g := range globs {
		if utf8.RuneCountInString(g) > maxGlobLen {
			return fmt.Errorf("sqlclass: secret table glob longer than %d characters", maxGlobLen)
		}
		if _, err := path.Match(g, ""); err != nil {
			return fmt.Errorf("sqlclass: secret table glob %q: %w", g, err)
		}
	}
	return nil
}
