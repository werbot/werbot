package pg

import (
	"bytes"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	pgq "github.com/pganalyze/pg_query_go/v6"
	pgquery "github.com/wasilibs/go-pgquery"

	"github.com/werbot/werbot/internal/platform/shellclass"
	"github.com/werbot/werbot/internal/platform/sqlclass"
)

// withEngine installs e as the package engine for the test.
func withEngine(t *testing.T, e *engine) {
	t.Helper()
	old := eng
	eng = e
	t.Cleanup(func() { eng = old })
}

// captureLog sends slog output to a buffer for the test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	return &buf
}

func isTransientOpaque(r sqlclass.Result) bool {
	return r.Transient && slices.Equal(r.Classes, []shellclass.Class{opaque})
}

// deepShapes are unparenthesised operator chains far beyond the guard bound
// (the first five) and deep nestings the guard does not count.
func deepShapes() []string {
	const n, nest = 8000, 50000
	return []string{
		"SELECT 1" + strings.Repeat(" + 1", n),
		"SELECT 'a'" + strings.Repeat(" || 'a'", n),
		"SELECT " + strings.Repeat("NOT ", n) + "true",
		"SELECT true" + strings.Repeat(" AND true", n),
		"SELECT " + strings.Repeat("- ", n) + "1",
		"SELECT " + strings.Repeat("(", nest) + "1" + strings.Repeat(")", nest),
		"SELECT " + strings.Repeat("abs(", nest) + "1" + strings.Repeat(")", nest),
		"SELECT " + strings.Repeat("ARRAY[", nest) + "1" + strings.Repeat("]", nest),
	}
}

func TestDeepExpressionIsOpaque(t *testing.T) {
	var parses atomic.Int32
	withEngine(t, &engine{scan: pgquery.Scan, parse: func(sql string) (*pgq.ParseResult, error) {
		parses.Add(1)
		return pgquery.Parse(sql)
	}})
	eng.ready.Store(true)
	for _, sql := range deepShapes()[:5] {
		r := classify(sql)
		if !slices.Equal(r.Classes, []shellclass.Class{opaque}) || r.Transient {
			t.Errorf("%.20q: classes %v transient %v, want deterministic opaque", sql, r.Classes, r.Transient)
		}
	}
	if n := parses.Load(); n != 0 {
		t.Fatalf("parser called %d times for inputs beyond the guard", n)
	}
	bound := "SELECT 1" + strings.Repeat(" + 1", maxOperatorTokens-1)
	if r := classify(bound); !slices.Equal(r.Classes, []shellclass.Class{read}) {
		t.Fatalf("input at the guard bound: classes %v, want read", r.Classes)
	}
}

func TestParserSurvivesDeepInput(t *testing.T) {
	for range 10 {
		for _, sql := range deepShapes() {
			if r := classify(sql); r.Transient {
				t.Fatalf("%.20q: transient result, the parser trapped", sql)
			}
		}
	}
	if Broken() {
		t.Fatal("deep inputs broke the parser")
	}
	for _, tc := range statementCases {
		if r := classify(tc.sql); !slices.Equal(r.Classes, tc.classes) || r.Transient {
			t.Errorf("%q after deep inputs: classes %v, want %v", tc.sql, r.Classes, tc.classes)
		}
	}
}

func panicking(string) (*pgq.ParseResult, error) { panic("wasm error: out of bounds memory access") }

func TestBrokenParserFailsClosed(t *testing.T) {
	logs := captureLog(t)
	withEngine(t, &engine{scan: pgquery.Scan, parse: panicking})
	eng.ready.Store(true)
	for range 3 {
		if r := classify("SELECT 1"); !isTransientOpaque(r) {
			t.Fatalf("broken parser: classes %v transient %v, want transient opaque", r.Classes, r.Transient)
		}
	}
	if !Broken() {
		t.Fatal("failing canary did not latch the parser as broken")
	}
	eng.parse = pgquery.Parse // the latch holds even if the parser seems to recover
	if r := classify("SELECT 1"); !isTransientOpaque(r) {
		t.Fatalf("latched parser: classes %v, want transient opaque", r.Classes)
	}
	if n := strings.Count(logs.String(), "level=ERROR"); n != 1 {
		t.Fatalf("broken parser logged %d errors, want 1", n)
	}
}

func TestTrapWithHealthyCanaryIsTransient(t *testing.T) {
	var calls atomic.Int32
	withEngine(t, &engine{scan: pgquery.Scan, parse: func(sql string) (*pgq.ParseResult, error) {
		if calls.Add(1) == 1 {
			panic("wasm trap")
		}
		return pgquery.Parse(sql)
	}})
	eng.ready.Store(true)
	if r := classify("SELECT 2"); !isTransientOpaque(r) {
		t.Fatalf("trapped parse: classes %v transient %v, want transient opaque", r.Classes, r.Transient)
	}
	if Broken() {
		t.Fatal("healthy canary latched the parser as broken")
	}
	if r := classify("SELECT 2"); !slices.Equal(r.Classes, []shellclass.Class{read}) {
		t.Fatalf("after a healthy canary: classes %v, want read", r.Classes)
	}
}

func TestScanTrapIsTransient(t *testing.T) {
	withEngine(t, &engine{parse: pgquery.Parse, scan: func(string) (*pgq.ScanResult, error) { panic("wasm trap") }})
	eng.ready.Store(true)
	if r := classify("SELECT 1"); !isTransientOpaque(r) || Broken() {
		t.Fatalf("scan trap: classes %v transient %v broken %v", r.Classes, r.Transient, Broken())
	}
}

func TestPoolBusyIsOpaque(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		old := sem
		sem = make(chan struct{}, 1)
		defer func() { sem = old }()
		sem <- struct{}{}
		start := time.Now()
		r := classify("SELECT 1")
		if !isTransientOpaque(r) {
			t.Fatalf("busy pool: classes %v transient %v, want transient opaque", r.Classes, r.Transient)
		}
		if d := time.Since(start); d != acquireTimeout {
			t.Fatalf("busy pool waited %v, want %v", d, acquireTimeout)
		}
	})
}

func TestClassifyBeforeWarmupIsTransientOpaque(t *testing.T) {
	release := make(chan struct{})
	e := &engine{scan: pgquery.Scan, parse: func(sql string) (*pgq.ParseResult, error) {
		if sql == canary {
			<-release // the warm-up parses only the canary
		}
		return pgquery.Parse(sql)
	}}
	withEngine(t, e)
	for range 2 {
		if r := classify("SELECT 2"); !isTransientOpaque(r) {
			t.Fatalf("before warm-up: classes %v transient %v, want transient opaque", r.Classes, r.Transient)
		}
	}
	close(release) // the first call started the warm-up in the background
	if err := e.warmup(); err != nil {
		t.Fatal(err)
	}
	if r := classify("SELECT 2"); !slices.Equal(r.Classes, []shellclass.Class{read}) {
		t.Fatalf("after warm-up: classes %v, want read", r.Classes)
	}
}

func TestWarmupErrorRefusesAll(t *testing.T) {
	logs := captureLog(t)
	e := &engine{scan: pgquery.Scan, parse: panicking}
	withEngine(t, e)
	err := Warmup()
	if err == nil || !strings.HasPrefix(err.Error(), "sql parser: ") {
		t.Fatalf("Warmup() = %v, want a sql parser error", err)
	}
	if again := Warmup(); !errors.Is(again, err) {
		t.Fatalf("second Warmup() = %v, want the first result", again)
	}
	if !Broken() {
		t.Fatal("failed warm-up did not latch the parser as broken")
	}
	e.parse = pgquery.Parse
	if r := classify("SELECT 1"); !isTransientOpaque(r) {
		t.Fatalf("after a failed warm-up: classes %v, want transient opaque", r.Classes)
	}
	if n := strings.Count(logs.String(), "level=ERROR"); n != 1 {
		t.Fatalf("failed warm-up logged %d errors, want 1", n)
	}
}
