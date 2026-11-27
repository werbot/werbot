// Package pg classifies PostgreSQL texts with libpg_query (go-pgquery, the
// PostgreSQL 17 grammar compiled to WebAssembly, no cgo). It registers the
// sqlclass.Postgres parser in init. The parser warms up lazily: the first
// call starts the warm-up in the background and every call before it ends is
// a transient opaque.
package pg

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	pgq "github.com/pganalyze/pg_query_go/v6"
	pgquery "github.com/wasilibs/go-pgquery"

	"github.com/werbot/werbot/internal/platform/sqlclass"
)

func init() { sqlclass.Register(sqlclass.Postgres, parser{}) }

// acquireTimeout bounds the wait for a parser instance.
const acquireTimeout = 100 * time.Millisecond

// sem bounds the parser instances in use: go-pgquery keeps them in its own
// unbounded pool, so the semaphore caps that pool at GOMAXPROCS.
var sem = make(chan struct{}, runtime.GOMAXPROCS(0))

// eng is the process-wide parser state; tests swap it.
var eng = &engine{parse: pgquery.Parse, scan: pgquery.Scan}

// engine holds the warm-up and the broken-parser latch around the library.
type engine struct {
	parse func(string) (*pgq.ParseResult, error)
	scan  func(string) (*pgq.ScanResult, error)

	warmOnce, kick sync.Once
	warmErr        error
	ready, broken  atomic.Bool
}

type parser struct{}

// Classify classifies sql. It never blocks longer than acquireTimeout for an
// instance; the parse itself is not interruptible (sqlclass.MaxText bounds it).
func (parser) Classify(sql string, _ sqlclass.Options) sqlclass.Result {
	return eng.classify(sql)
}

// Warmup compiles the parser and fills the instance pool. It is idempotent:
// a second call returns the first result. A failure latches the parser as
// broken, so every later classification is a transient opaque.
func Warmup() error { return eng.warmup() }

// Broken reports whether the parser is unusable until the process restarts.
func Broken() bool { return eng.broken.Load() }

func (e *engine) classify(sql string) sqlclass.Result {
	if e.broken.Load() || !e.isReady() || !acquire() {
		return sqlclass.Transient()
	}
	defer func() { <-sem }()
	scan, err := guarded(e.scan, sql)
	if isTrap(err) {
		return e.trapped()
	}
	if err != nil || tooDeep(scan) {
		return sqlclass.Opaque()
	}
	tree, err := guarded(e.parse, sql)
	if isTrap(err) {
		return e.trapped()
	}
	if err != nil {
		return sqlclass.Opaque()
	}
	return classes(tree)
}

// isReady reports a finished warm-up; before that it starts the warm-up in
// the background once.
func (e *engine) isReady() bool {
	if e.ready.Load() {
		return true
	}
	e.kick.Do(func() { go func() { _ = e.warmup() }() })
	return false
}

func acquire() bool {
	select {
	case sem <- struct{}{}:
		return true
	default:
	}
	t := time.NewTimer(acquireTimeout)
	defer t.Stop()
	select {
	case sem <- struct{}{}:
		return true
	case <-t.C:
		return false
	}
}
