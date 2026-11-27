package pg

import (
	"cmp"
	"slices"
	"testing"

	pgquery "github.com/wasilibs/go-pgquery"

	"github.com/werbot/werbot/internal/platform/shellclass"
	"github.com/werbot/werbot/internal/platform/sqlclass"
)

// fuzzMax skips larger inputs: they only slow the run down, the guard and
// sqlclass.MaxText bound them.
const fuzzMax = 1 << 16

// FuzzSQLClass: no panic; classes non-empty, ascending, valid; tables sorted
// and unique, write tables among them; the same result twice; never
// transient (a transient result is a parser trap: a finding); a text the
// parser rejects is [opaque]; "a; b" keeps every class and table of "a".
func FuzzSQLClass(f *testing.F) {
	rows := loadCorpus(f)
	for i, r := range rows {
		f.Add(r.sql, rows[(i+1)%len(rows)].sql)
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		if len(a) > fuzzMax || len(b) > fuzzMax {
			t.Skip()
		}
		ra := check(t, a)
		if ra.Statements == 0 || slices.Contains(ra.Classes, shellclass.Opaque) {
			return
		}
		ab := check(t, a+"; "+b)
		if slices.Contains(ab.Classes, shellclass.Opaque) {
			return
		}
		if !subset(ra.Classes, ab.Classes) || !subset(ra.Tables, ab.Tables) || !subset(ra.WriteTables, ab.WriteTables) {
			t.Fatalf("%q then %q: %v lost parts of %v", a, b, ab, ra)
		}
	})
}

// check classifies sql twice and checks the result invariants.
func check(t *testing.T, sql string) sqlclass.Result {
	t.Helper()
	r := classify(sql)
	if r.Transient {
		t.Fatalf("%q: transient result (parser trap or broken parser)", sql)
	}
	if len(r.Classes) == 0 || !sortedUnique(r.Classes) || slices.ContainsFunc(r.Classes, invalid) {
		t.Fatalf("%q: classes %v, want ascending valid classes without repeats", sql, r.Classes)
	}
	if !validTables(r) {
		t.Fatalf("%q: tables %v write tables %v, want sorted unique, writes among tables", sql, r.Tables, r.WriteTables)
	}
	if again := classify(sql); !equal(r, again) {
		t.Fatalf("%q: %v then %v", sql, r, again)
	}
	if !slices.Equal(r.Classes, []shellclass.Class{shellclass.Opaque}) && parseFails(sql) {
		t.Fatalf("%q: the parser rejects it, classes %v, want [opaque]", sql, r.Classes)
	}
	return r
}

// parseFails parses sql directly; inputs beyond the guard are not parsed.
func parseFails(sql string) bool {
	s, err := pgquery.Scan(sql)
	if err != nil || tooDeep(s) {
		return true
	}
	_, err = pgquery.Parse(sql)
	return err != nil
}

func equal(a, b sqlclass.Result) bool {
	return slices.Equal(a.Classes, b.Classes) && slices.Equal(a.Tables, b.Tables) &&
		slices.Equal(a.WriteTables, b.WriteTables) && a.Statements == b.Statements && a.Transient == b.Transient
}

// validTables: both lists sorted and unique, every write table also a table.
func validTables(r sqlclass.Result) bool {
	return sortedUnique(r.Tables) && sortedUnique(r.WriteTables) && subset(r.WriteTables, r.Tables)
}

func invalid(c shellclass.Class) bool { return !shellclass.Valid(c) }

func sortedUnique[T cmp.Ordered](s []T) bool {
	return slices.IsSorted(s) && len(slices.Compact(slices.Clone(s))) == len(s)
}

func subset[T comparable](sub, of []T) bool {
	for _, x := range sub {
		if !slices.Contains(of, x) {
			return false
		}
	}
	return true
}
