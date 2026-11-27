package sqlclass

import (
	"slices"
	"strings"
	"testing"

	"github.com/werbot/werbot/internal/platform/shellclass"
)

// fakeParser returns a fixed result, or panics when panics is set.
type fakeParser struct {
	result Result
	panics bool
	calls  int
}

func (f *fakeParser) Classify(string, Options) Result {
	f.calls++
	if f.panics {
		panic("parser failure")
	}
	return f.result
}

// register installs p under a dialect unique to the test.
func register(t *testing.T, p Parser) Dialect {
	t.Helper()
	d := Dialect("test-" + t.Name())
	Register(d, p)
	t.Cleanup(func() { delete(parsers, d) })
	return d
}

func isOpaque(r Result) bool {
	return slices.Equal(r.Classes, []shellclass.Class{shellclass.Opaque})
}

func TestClassifyWithoutParserIsOpaque(t *testing.T) {
	r := Classify("bogus", "SELECT 1", Options{})
	if !isOpaque(r) || r.Statements != 0 || !r.Transient {
		t.Fatalf("got %+v, want transient [opaque] with 0 statements", r)
	}
}

func TestClassifyTooLongIsOpaque(t *testing.T) {
	p := &fakeParser{result: Result{Classes: []shellclass.Class{shellclass.Read}, Statements: 1}}
	d := register(t, p)
	r := Classify(d, strings.Repeat("x", MaxText+1), Options{})
	if !isOpaque(r) || !r.Transient {
		t.Fatalf("got %+v, want transient [opaque]", r)
	}
	if p.calls != 0 {
		t.Fatal("parser called for a text over MaxText")
	}
	if r := Classify(d, strings.Repeat("x", MaxText), Options{}); isOpaque(r) {
		t.Fatalf("text of exactly MaxText: got %+v, want the parser result", r)
	}
}

func TestClassifyPanicIsOpaque(t *testing.T) {
	d := register(t, &fakeParser{panics: true})
	r := Classify(d, "SELECT 1", Options{SecretTables: []string{"*"}})
	if !isOpaque(r) || !r.Transient {
		t.Fatalf("got %+v, want transient [opaque]", r)
	}
}

func TestClassifyEmptyResultIsOpaque(t *testing.T) {
	d := register(t, &fakeParser{result: Result{Statements: 1}})
	r := Classify(d, "SELECT 1", Options{})
	if !isOpaque(r) {
		t.Fatalf("got %+v, want [opaque]", r)
	}
}

// TestTransientOpaque: busy, not-ready or broken parsers (reported by the
// dialect as Transient), panics, too-long texts and missing parsers give a
// transient [opaque]; a deterministic parse error stays non-transient.
func TestTransientOpaque(t *testing.T) {
	read := []shellclass.Class{shellclass.Read}
	cases := []struct {
		name      string
		parser    Parser
		dialect   Dialect // set → no parser registered
		text      string
		transient bool
	}{
		{name: "busy or not ready", parser: &fakeParser{result: Result{Transient: true}}, transient: true},
		{name: "transient with classes", parser: &fakeParser{result: Result{Classes: read, Tables: []string{"public.t"}, Statements: 1, Transient: true}}, transient: true},
		{name: "panic", parser: &fakeParser{panics: true}, transient: true},
		{name: "too long", parser: &fakeParser{result: Result{Classes: read}}, text: strings.Repeat("x", MaxText+1), transient: true},
		{name: "no parser", dialect: "bogus", transient: true},
		{name: "parse error", parser: &fakeParser{result: Opaque()}, transient: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := tc.dialect
			if tc.parser != nil {
				d = register(t, tc.parser)
			}
			text := tc.text
			if text == "" {
				text = "SELECT 1"
			}
			r := Classify(d, text, Options{})
			if !isOpaque(r) || r.Transient != tc.transient {
				t.Fatalf("got %+v, want [opaque] transient=%v", r, tc.transient)
			}
			if r.Tables != nil || r.WriteTables != nil {
				t.Fatalf("got tables %v %v, want none", r.Tables, r.WriteTables)
			}
		})
	}
}

func TestClassifyPassesResultAndMarksSecrets(t *testing.T) {
	want := Result{Classes: []shellclass.Class{shellclass.Read}, Tables: []string{"billing.cards"}, Statements: 1}
	d := register(t, &fakeParser{result: want})
	r := Classify(d, "SELECT 1", Options{})
	if !slices.Equal(r.Classes, want.Classes) || r.Transient || r.Statements != 1 {
		t.Fatalf("got %+v, want %+v", r, want)
	}
	r = Classify(d, "SELECT 1", Options{SecretTables: []string{"billing.*"}})
	if !slices.Equal(r.Classes, []shellclass.Class{shellclass.Read, shellclass.Secrets}) {
		t.Fatalf("got %v, want [read secrets]", r.Classes)
	}
}

func TestMarkSecrets(t *testing.T) {
	in := Result{
		Classes:     []shellclass.Class{shellclass.Read, shellclass.Write},
		Tables:      []string{"public.users"},
		WriteTables: []string{"billing.cards"},
		Statements:  1,
	}
	orig := slices.Clone(in.Classes)

	got := MarkSecrets(in, []string{"billing.*"})
	want := []shellclass.Class{shellclass.Read, shellclass.Secrets, shellclass.Write}
	if !slices.Equal(got.Classes, want) {
		t.Fatalf("got %v, want %v", got.Classes, want)
	}
	if !slices.Equal(in.Classes, orig) {
		t.Fatal("MarkSecrets changed the input classes")
	}
	if got := MarkSecrets(in, []string{"*.tokens"}); !slices.Equal(got.Classes, orig) {
		t.Fatalf("no match: got %v, want %v", got.Classes, orig)
	}
	if got := MarkSecrets(in, nil); !slices.Equal(got.Classes, orig) {
		t.Fatalf("no globs: got %v, want %v", got.Classes, orig)
	}
}

func TestMerge(t *testing.T) {
	got := Merge([]shellclass.Class{shellclass.Write, shellclass.Read}, []shellclass.Class{shellclass.Read, shellclass.Destructive})
	want := []shellclass.Class{shellclass.Destructive, shellclass.Read, shellclass.Write}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestRegisterTwicePanics(t *testing.T) {
	d := register(t, &fakeParser{})
	defer func() {
		if recover() == nil {
			t.Fatal("second Register did not panic")
		}
	}()
	Register(d, &fakeParser{})
}

func TestProtected(t *testing.T) {
	for _, name := range []string{"Statement_Timeout", "search_path", "standard_conforming_strings", "CLIENT_ENCODING", "backslash_quote"} {
		if !Protected(name) {
			t.Errorf("Protected(%q) = false, want true", name)
		}
	}
	if Protected("work_mem") {
		t.Error(`Protected("work_mem") = true, want false`)
	}
}

func TestValidGlobs(t *testing.T) {
	if err := ValidGlobs([]string{"["}); err == nil {
		t.Error(`"[": want error`)
	}
	if err := ValidGlobs(make([]string, 65)); err == nil {
		t.Error("65 globs: want error")
	}
	if err := ValidGlobs([]string{strings.Repeat("a", 256)}); err == nil {
		t.Error("256 characters: want error")
	}
	if err := ValidGlobs([]string{"public.*", strings.Repeat("a", 255)}); err != nil {
		t.Errorf("valid globs: %v", err)
	}
}
