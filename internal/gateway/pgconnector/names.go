package pgconnector

import (
	"bytes"
	"strings"
	"sync/atomic"

	"github.com/werbot/werbot/internal/gateway/dbproxy"
)

// The database's answer to the Parse or Bind that made a name.
const (
	namePending int32 = iota
	nameMade
	nameFailed
)

// named is a prepared statement or a portal: its logged text (a portal's is
// its statement's), its statement and logged parameters (a portal's), and
// the bytes it keeps of its own. made holds the database's answer to the
// message that made it, sent in the client's cycle (Syncs passed) cycle. A
// portal's src is the made of its statement at the Bind; nil when the
// gateway could not tell which statement the database bound.
type named struct {
	stmt, text string
	params     []string
	size       int
	made, src  *atomic.Int32
	cycle      uint64
}

// store keeps v under name in m (c.stmts or c.portals) within the limits of
// a connection: maxNames names and maxRetained bytes in all. v.size is what
// v keeps besides its name. A name that is there already is replaced. The
// answer to the message that makes v is c.made until the next store.
// ponytail: a named portal the database drops at transaction end without a
// Close still counts; drop them on ReadyForQuery 'I' if clients hit a limit.
func (c *clientSide) store(m map[string]named, name string, v named) error {
	v.size += len(name)
	old, ok := m[name]
	if !ok && len(c.stmts)+len(c.portals) >= maxNames {
		return errTooManyNames
	}
	kept := c.kept + v.size - old.size
	if kept > maxRetained {
		return errRetained
	}
	v.made, v.cycle = new(atomic.Int32), c.cycles
	m[name], c.kept, c.made = v, kept, v.made
	return nil
}

// drop forgets a closed statement or portal and the bytes it kept.
func (c *clientSide) drop(m map[string]named, name string) {
	c.kept -= m[name].size
	delete(m, name)
}

// dropStmt forgets a statement here and in the guard.
func (c *clientSide) dropStmt(name string) {
	c.drop(c.stmts, name)
	c.g.Forget(name)
}

// known reports whether the database holds v for the next message: the
// message that made it succeeded, or is still pending in this cycle (an
// error before the next message skips both).
func (c *clientSide) known(v named) bool {
	switch v.made.Load() {
	case nameMade:
		return true
	case namePending:
		return v.cycle == c.cycles
	}
	return false
}

// parsed keeps the text of a new statement. A Parse to a non-empty name
// that is there keeps it: the database refuses that Parse (42P05). A name
// whose Parse the database refused is free.
func (c *clientSide) parsed(body []byte) (*item, error) {
	if c.parse.Decode(body) != nil {
		return nil, errMalformed
	}
	name := c.parse.Name
	old, ok := c.stmts[name]
	if ok && old.made.Load() == nameFailed {
		c.dropStmt(name)
		ok = false
	}
	c.made = nil
	if !ok || name == "" {
		text := dbproxy.LogText(c.parse.Query)
		if err := c.store(c.stmts, name, named{text: text, size: len(text)}); err != nil {
			return nil, err
		}
	}
	return c.checked('P', dbproxy.Op{Kind: "parse", Text: c.parse.Query, Statement: name, DB: c.log.db})
}

// bound keeps a new portal with its displayed parameters, which an approval
// of its execute covers. The portal's statement is known only when the
// database holds it for this Bind, and never for a Bind to a non-empty
// portal name that is there (the database refuses it with 42P03, or took it
// once the transaction that held the old one ended). A failed Bind is logged
// as its execute, with the parameters when the resource records them.
func (c *clientSide) bound(name, stmt string, params []string) (*item, error) {
	p := named{stmt: stmt, params: params}
	if s, ok := c.stmts[stmt]; ok && c.known(s) {
		p.text, p.src = s.text, s.made
	}
	if old, ok := c.portals[name]; ok && name != "" && old.made.Load() != nameFailed {
		p.src = nil
	}
	for _, v := range params {
		p.size += len(v)
	}
	if err := c.store(c.portals, name, p); err != nil {
		return nil, err
	}
	it := c.newItem('B', dbproxy.Op{Kind: "execute", Text: p.text, Statement: stmt})
	it.made = c.made
	if c.log.params {
		it.entry.Params = params
	}
	return it, nil
}

// executeOp is the operation of an Execute of portal name: its statement's
// text and its parameters while the database holds the portal and its
// statement is still the one it was bound from; else opaque without a text,
// which no approval covers.
func (c *clientSide) executeOp(name string) dbproxy.Op {
	p, ok := c.portals[name]
	op := dbproxy.Op{Kind: "execute", Statement: p.stmt, DB: c.log.db}
	if ok && p.src != nil && c.known(p) && c.stmts[p.stmt].made == p.src {
		op.Text, op.Params = p.text, p.params
	} else {
		op.Opaque = true
	}
	return op
}

// closed forgets a closed statement or portal.
func (c *clientSide) closed(body []byte) (*item, error) {
	if c.close.Decode(body) != nil {
		return nil, errMalformed
	}
	switch c.close.ObjectType {
	case 'S':
		c.dropStmt(c.close.Name)
	case 'P':
		c.drop(c.portals, c.close.Name)
	}
	return &item{kind: 'C'}, nil
}

// syncNames forgets every named statement once SQL PREPARE, DEALLOCATE or
// DISCARD reached the database since the last look.
func (c *clientSide) syncNames() {
	n := c.resets.Load()
	if n == c.resetsSeen {
		return
	}
	c.resetsSeen = n
	for name := range c.stmts {
		if name != "" {
			c.dropStmt(name)
		}
	}
}

// renameWords start the SQL statements that change the prepared statements.
var renameWords = []string{"PREPARE", "DEALLOCATE", "DISCARD"}

// renames reports whether text may hold one of renameWords: the next cycle
// then waits for its reply, whose tag tells whether one ran.
func renames(text string) bool {
	for i := range len(text) {
		if b := text[i] | 0x20; b != 'p' && b != 'd' {
			continue
		}
		for _, w := range renameWords {
			if len(text)-i >= len(w) && strings.EqualFold(text[i:i+len(w)], w) {
				return true
			}
		}
	}
	return false
}

// renamed reports whether a CommandComplete tag is of one of renameWords.
func renamed(tag []byte) bool {
	if len(tag) == 0 || (tag[0] != 'P' && tag[0] != 'D') {
		return false
	}
	for _, w := range renameWords {
		if bytes.HasPrefix(tag, []byte(w)) {
			return true
		}
	}
	return false
}
