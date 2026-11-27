package pg

import (
	"bufio"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/werbot/werbot/internal/platform/shellclass"
	"github.com/werbot/werbot/internal/platform/sqlclass"
)

// corpusRow is one line of testdata/corpus.txt.
type corpusRow struct {
	line                int
	classes             []shellclass.Class
	tables, writeTables []string
	sql                 string
}

// loadCorpus reads testdata/corpus.txt; blank lines and "#" lines are skipped.
func loadCorpus(tb testing.TB) []corpusRow {
	tb.Helper()
	f, err := os.Open("testdata/corpus.txt")
	if err != nil {
		tb.Fatal(err)
	}
	defer f.Close()
	var rows []corpusRow
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.SplitN(line, " | ", 4)
		if len(cols) != 4 {
			tb.Fatalf("corpus line %d: want 4 columns", n)
		}
		var classes []shellclass.Class
		for _, c := range list(cols[0]) {
			classes = append(classes, shellclass.Class(c))
		}
		slices.Sort(classes)
		rows = append(rows, corpusRow{n, classes, list(cols[1]), list(cols[2]), strings.ReplaceAll(cols[3], `\n`, "\n")})
	}
	if err := sc.Err(); err != nil {
		tb.Fatal(err)
	}
	return rows
}

// list splits a comma-separated corpus column; "-" is the empty list.
func list(col string) []string {
	if col == "-" {
		return nil
	}
	out := strings.Split(col, ", ")
	slices.Sort(out)
	return out
}

func TestCorpus(t *testing.T) {
	rows := loadCorpus(t)
	if len(rows) < 60 {
		t.Fatalf("corpus has %d lines, want at least 60", len(rows))
	}
	for _, row := range rows {
		r := classify(row.sql)
		if r.Transient || !slices.Equal(r.Classes, row.classes) ||
			!slices.Equal(r.Tables, row.tables) || !slices.Equal(r.WriteTables, row.writeTables) {
			t.Errorf("corpus line %d %q:\n got %v | %v | %v (transient %v)\nwant %v | %v | %v",
				row.line, row.sql, r.Classes, r.Tables, r.WriteTables, r.Transient, row.classes, row.tables, row.writeTables)
		}
	}
}

// protectedForms changes the protected parameter name by every statement form.
func protectedForms(name string) []string {
	upper := strings.ToUpper(name)
	return []string{
		"SET " + name + " = 'x'",
		"SET LOCAL " + name + " = 'x'",
		"SET SESSION " + name + " TO 'x'",
		`SET "` + upper + `" = 'x'`,
		"SET " + name + " TO DEFAULT",
		"RESET " + name,
		"SELECT set_config('" + name + "', 'x', false)",
		"SELECT pg_catalog.set_config('" + upper + "', 'x', true)",
		"SELECT 1 FROM t WHERE set_config('" + name + "', 'x', false) IS NOT NULL",
		"UPDATE pg_settings SET setting = 'x' WHERE name = '" + name + "'",
	}
}

func TestProtectedParams(t *testing.T) {
	var texts []string
	for _, name := range sqlclass.ProtectedParams {
		texts = append(texts, protectedForms(name)...)
	}
	texts = append(texts,
		"SET ROLE admin", "SET LOCAL ROLE admin", "RESET ROLE", "SET SESSION AUTHORIZATION admin",
		"RESET SESSION AUTHORIZATION", "SET SCHEMA 'x'", "SET NAMES 'SJIS'",
		"RESET ALL", "DISCARD ALL", "SELECT set_config(current_user, 'x', false)",
		"BEGIN READ WRITE", "START TRANSACTION READ WRITE", "SET TRANSACTION READ WRITE",
		"SET SESSION CHARACTERISTICS AS TRANSACTION READ WRITE",
	)
	for _, sql := range texts {
		if r := classify(sql); !slices.Contains(r.Classes, privilege) || r.Transient {
			t.Errorf("%q: classes %v transient %v, want privilege", sql, r.Classes, r.Transient)
		}
	}
	for _, sql := range []string{
		"SET work_mem = '64MB'", "SELECT set_config('work_mem', '64MB', false)", "RESET work_mem",
		"BEGIN READ ONLY", "SET TRANSACTION READ ONLY", "SELECT current_setting('search_path')",
	} {
		if r := classify(sql); !slices.Equal(r.Classes, []shellclass.Class{read}) {
			t.Errorf("%q: classes %v, want read", sql, r.Classes)
		}
	}
}

func TestTablesSortedUnique(t *testing.T) {
	r := classify("SELECT * FROM b JOIN a ON true JOIN b b2 ON true")
	if want := []string{"public.a", "public.b"}; !slices.Equal(r.Tables, want) || r.WriteTables != nil {
		t.Fatalf("tables %v write tables %v, want %v and none", r.Tables, r.WriteTables, want)
	}
}
