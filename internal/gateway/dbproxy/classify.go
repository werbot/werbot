package dbproxy

import (
	"container/list"
	"crypto/sha256"
	"strings"
	"sync"

	"github.com/werbot/werbot/internal/platform/sqlclass"
)

// Dialect classifies the operation texts of one resource type.
type Dialect interface {
	Classify(text string, opt sqlclass.Options) sqlclass.Result
}

type sqlDialect sqlclass.Dialect

func (d sqlDialect) Classify(text string, opt sqlclass.Options) sqlclass.Result {
	return sqlclass.Classify(sqlclass.Dialect(d), text, opt)
}

// SQLDialect is the Dialect of d through sqlclass.Classify.
func SQLDialect(d sqlclass.Dialect) Dialect { return sqlDialect(d) }

// classCacheSize is the number of texts the class cache of one type keeps.
const classCacheSize = 4096

// classified is a classification with its Verdict and AccessRequest forms,
// computed once. Its slices are shared: nobody modifies them.
type classified struct {
	res                     sqlclass.Result
	classes, tables, writes []string
	key                     string // classes, tables and write tables: the decision cache key
}

func newClassified(r sqlclass.Result) *classified {
	c := &classified{res: r, tables: r.Tables, writes: r.WriteTables, classes: make([]string, len(r.Classes))}
	for i, cl := range r.Classes {
		c.classes[i] = string(cl)
	}
	c.key = strings.Join(c.classes, ",") + "\x00" + strings.Join(c.tables, ",") + "\x00" + strings.Join(c.writes, ",")
	return c
}

// opaque is the deterministic fail-closed classification.
var opaque = newClassified(sqlclass.Opaque())

// classCache is an LRU of classifications by the SHA-256 of the text, shared
// by the sessions of one type. Transient results never enter it.
type classCache struct {
	mu sync.Mutex
	ll list.List // of *cacheEntry, most recent first
	m  map[[32]byte]*list.Element
}

type cacheEntry struct {
	key [32]byte
	c   *classified
}

func (c *classCache) get(k [32]byte) *classified {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[k]
	if !ok {
		return nil
	}
	c.ll.MoveToFront(e)
	return e.Value.(*cacheEntry).c
}

func (c *classCache) put(k [32]byte, v *classified) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.m[k]; ok {
		e.Value.(*cacheEntry).c = v
		c.ll.MoveToFront(e)
		return
	}
	c.m[k] = c.ll.PushFront(&cacheEntry{key: k, c: v})
	if c.ll.Len() > classCacheSize {
		last := c.ll.Back()
		c.ll.Remove(last)
		delete(c.m, last.Value.(*cacheEntry).key)
	}
}

// classCaches holds the class cache of each type of this gateway.
var classCaches = struct {
	sync.Mutex
	m map[string]*classCache
}{m: map[string]*classCache{}}

func classCacheOf(typ string) *classCache {
	classCaches.Lock()
	defer classCaches.Unlock()
	c := classCaches.m[typ]
	if c == nil {
		c = &classCache{m: map[[32]byte]*list.Element{}}
		classCaches.m[typ] = c
	}
	return c
}

// classify is the classification of text (SHA-256 sha) with the session's
// secret tables: from the shared cache, else from the dialect, cached unless
// transient. Secret tables are marked after the cache, which is shared.
func (g *Guard) classify(text string, sha [32]byte) *classified {
	c := g.classes.get(sha)
	if c != nil {
		g.hits.Inc()
	} else {
		c = newClassified(g.dialect.Classify(text, sqlclass.Options{}))
		if !c.res.Transient {
			g.classes.put(sha, c)
		}
		g.noteBroken()
	}
	if len(g.secretTables) == 0 {
		return c
	}
	if r := sqlclass.MarkSecrets(c.res, g.secretTables); len(r.Classes) != len(c.res.Classes) {
		return newClassified(r)
	}
	return c
}

// noteBroken sets the parser gauge from the dialect's broken latch.
func (g *Guard) noteBroken() {
	if g.brokenFn == nil {
		return
	}
	var v int64
	if g.brokenFn() {
		v = 1
	}
	g.broken.Set(v)
}

// logClasses fills the classes and tables of a querylog entry of the fast
// path, which Check did not classify.
func (g *Guard) logClasses(e *Entry) {
	if e.Classes != nil || g.dialect == nil || e.Text == "" || (e.Kind != "query" && e.Kind != "execute" && e.Kind != "request") {
		return
	}
	c := g.classify(e.Text, sha256.Sum256([]byte(e.Text)))
	e.Classes, e.Tables = c.classes, c.tables
}
