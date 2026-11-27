package pg

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	pgq "github.com/pganalyze/pg_query_go/v6"

	"github.com/werbot/werbot/internal/platform/sqlclass"
)

// maxOperatorTokens bounds the operator and expression-keyword tokens of a
// text. Long unparenthesised operator chains overflow the stack of the wasm
// parser, and a few such traps corrupt its heap shared by every instance.
const maxOperatorTokens = 2000

// canary is parsed after a trap to check that the shared heap still works.
const canary = "SELECT 1"

// trapError is a panic of the wasm runtime turned into an error.
type trapError struct{ v any }

func (e trapError) Error() string { return fmt.Sprint(e.v) }

func isTrap(err error) bool { return errors.As(err, new(trapError)) }

// guarded calls f and turns its panic into a trapError.
func guarded[T any](f func(string) (T, error), sql string) (res T, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = trapError{r}
		}
	}()
	return f(sql)
}

// tooDeep reports more than maxOperatorTokens operator tokens: everything
// but identifiers, constants, parameters, comments, ", ; ( ) ." and keywords
// that may be names (unreserved and column-name keywords).
func tooDeep(s *pgq.ScanResult) bool {
	n := 0
	for _, t := range s.GetTokens() {
		if counted(t) {
			n++
		}
	}
	return n > maxOperatorTokens
}

func counted(t *pgq.ScanToken) bool {
	switch t.GetKeywordKind() {
	case pgq.KeywordKind_UNRESERVED_KEYWORD, pgq.KeywordKind_COL_NAME_KEYWORD:
		return false
	case pgq.KeywordKind_NO_KEYWORD:
		return !uncounted[t.GetToken()]
	}
	return true
}

var uncounted = map[pgq.Token]bool{
	pgq.Token_IDENT: true, pgq.Token_UIDENT: true, pgq.Token_ICONST: true, pgq.Token_FCONST: true,
	pgq.Token_SCONST: true, pgq.Token_USCONST: true, pgq.Token_BCONST: true, pgq.Token_XCONST: true,
	pgq.Token_PARAM: true, pgq.Token_SQL_COMMENT: true, pgq.Token_C_COMMENT: true,
	pgq.Token_ASCII_40: true, pgq.Token_ASCII_41: true, pgq.Token_ASCII_44: true,
	pgq.Token_ASCII_46: true, pgq.Token_ASCII_59: true,
}

// trapped answers a trapped call and checks the parser with the canary; a
// failing canary latches the parser as broken.
// ponytail: the latch holds until restart; a restart restores the parser.
func (e *engine) trapped() sqlclass.Result {
	if err := e.check(); err != nil {
		e.latch("canary parse failed", err)
	}
	return transient()
}

func (e *engine) check() error {
	tree, err := guarded(e.parse, canary)
	if err == nil && len(tree.GetStmts()) != 1 {
		err = errors.New("canary parse returned a wrong tree")
	}
	return err
}

// latch marks the parser broken and logs it once.
func (e *engine) latch(reason string, err error) {
	if e.broken.CompareAndSwap(false, true) {
		slog.Error("sql parser broken until restart", "type", sqlclass.Postgres, "reason", reason, "error", err)
	}
}

func (e *engine) warmup() error {
	e.warmOnce.Do(func() {
		if e.warmErr = e.compile(); e.warmErr != nil {
			e.latch("warm-up failed", e.warmErr)
			return
		}
		e.ready.Store(true)
	})
	return e.warmErr
}

// compile runs the first parse (it compiles the wasm module), then one
// parse per sem slot in parallel so the pool holds its instances.
func (e *engine) compile() error {
	if err := e.check(); err != nil {
		return fmt.Errorf("sql parser: %w", err)
	}
	errs := make([]error, cap(sem))
	var wg sync.WaitGroup
	for i := range errs {
		wg.Go(func() { errs[i] = e.check() })
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("sql parser: %w", err)
	}
	return nil
}
